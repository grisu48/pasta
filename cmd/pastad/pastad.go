/*
 * pasted - stupid simple paste server
 */

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	BaseUrl         string `toml:"BaseURL"`  // Instance base URL
	PastaDir        string `toml:"PastaDir"` // dir where pasta are stored
	BindAddr        string `toml:"BindAddress"`
	MaxPastaSize    int64  `toml:"MaxPastaSize"` // Max bin size in bytes
	PastaCharacters int    `toml:"PastaCharacters"`
	MimeTypesFile   string `toml:"MimeTypes` // Load mime types from this file
	DefaultExpire   int64  `toml:"Expire"`   // Default TTL for a new pasta in seconds
	CleanupInterval int    `toml:"Cleanup"`  // Seconds between cleanup cycles
}

var cf Config
var bowl PastaBowl
var mimeExtensions map[string]string

func ExtractPastaId(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	} else {
		return path[i+1:]
	}
}

/* Load MIME types file. MIME types file is a simple text file that describes mime types based on file extenstions.
 * The format of the file is
 * EXTENSION = MIMETYPE
 */
func loadMimeTypes(filename string) (map[string]string, error) {
	ret := make(map[string]string, 0)

	file, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return ret, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			continue
		}
		name, value := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
		if name != "" && value != "" {
			ret[name] = value
		}
	}

	return ret, scanner.Err()
}

func takeFirst(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	return arr[0]
}

func SendPasta(pasta Pasta, w http.ResponseWriter) error {
	file, err := bowl.GetPastaReader(pasta.Id)
	if err != nil {
		return err
	}
	defer file.Close()
	w.Header().Set("Content-Length", strconv.FormatInt(pasta.Size, 10))
	if pasta.Mime != "" {
		w.Header().Set("Content-Type", pasta.Mime)
	}
	_, err = io.Copy(w, file)
	return err
}

func deletePasta(id string, token string, w http.ResponseWriter) {
	var pasta Pasta
	var err error
	if id == "" || token == "" {
		goto Invalid
	}
	pasta, err = bowl.GetPasta(id)
	if err != nil {
		log.Fatalf("Error getting pasta %s: %s", pasta.Id, err)
		goto ServerError
	}
	if pasta.Id == "" {
		goto NotFound
	}
	if pasta.Token == token {
		err = bowl.DeletePasta(pasta.Id)
		if err != nil {
			log.Fatalf("Error deleting pasta %s: %s", pasta.Id, err)
			goto ServerError
		}
		fmt.Fprintf(w, "OK")
	} else {
		goto Invalid
	}
	return
NotFound:
	w.WriteHeader(404)
	fmt.Fprintf(w, "pasta not found")
	return
Invalid:
	w.WriteHeader(403)
	fmt.Fprintf(w, "Invalid request")
	return
ServerError:
	w.WriteHeader(500)
	fmt.Fprintf(w, "server error")
}

func receiveBody(reader io.Reader, pasta *Pasta) error {
	buf := make([]byte, 4096)
	file, err := os.OpenFile(pasta.Filename, os.O_RDWR|os.O_APPEND, 0640)
	if err != nil {
		file.Close()
		return err
	}
	defer file.Close()
	pasta.Size = 0
	for pasta.Size < cf.MaxPastaSize {
		n, err := reader.Read(buf)
		if (err == nil || err == io.EOF) && n > 0 {
			if _, err = file.Write(buf[:n]); err != nil {
				log.Fatalf("Write error while receiving bin: %s", err)
				return err
			}
			pasta.Size += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Fatalf("Receive error while receiving bin: %s", err)
			return err
		}
	}
	return nil
}

/* try to determine the mime type by file extension. Returns empty string on failure */
func mimeByFilename(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i < 0 {
		return ""
	}
	extension := filename[i+1:]
	if mime, ok := mimeExtensions[extension]; ok {
		return mime
	}
	return ""
}

func receiveMultibody(r *http.Request, pasta *Pasta) (io.ReadCloser, error) {
	err := r.ParseMultipartForm(cf.MaxPastaSize)
	if err != nil {
		return nil, err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	filename := header.Filename
	// Determine MIME type based on file extension, if present
	if filename != "" {
		pasta.Mime = mimeByFilename(filename)
	}
	return file, err
}

/* Parse expire header value. Returns expire value or 0 on error or invalid settings */
func parseExpire(headerValue []string) int64 {
	var ret int64
	for _, value := range headerValue {
		if expire, err := strconv.ParseInt(value, 10, 64); err == nil {
			// No negative values allowed
			if expire < 0 {
				return 0
			}
			ret = time.Now().Unix() + int64(expire)
		}
	}
	return ret
}

/* isMultipart returns true if the given request is multipart form */
func isMultipart(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return contentType == "multipart/form-data" || strings.HasPrefix(contentType, "multipart/form-data;")
}

func ReceivePasta(r *http.Request) (Pasta, error) {
	var err error
	var reader io.ReadCloser
	pasta := Pasta{Id: ""}

	// Pase expire if given
	if cf.DefaultExpire > 0 {
		pasta.ExpireDate = time.Now().Unix() + cf.DefaultExpire
	}
	if expire := parseExpire(r.Header["Expire"]); expire > 0 {
		pasta.ExpireDate = expire
	}

	pasta.Id = bowl.GenerateRandomBinId(cf.PastaCharacters)
	// InsertPasta sets filename
	if err = bowl.InsertPasta(&pasta); err != nil {
		return pasta, err
	}

	if isMultipart(r) {
		// Close body, also if it's not set as reader
		defer r.Body.Close()
		reader, err = receiveMultibody(r, &pasta)
		if err != nil {
			return pasta, err
		}
	} else {
		// Otherwise the message body is the upload content
		reader = r.Body
	}
	defer reader.Close()

	if err := receiveBody(reader, &pasta); err != nil {
		return pasta, err
	}
	if pasta.Size >= cf.MaxPastaSize {
		log.Println("Max size exceeded while receiving bin")
		return pasta, errors.New("Bin size exceeded")
	}
	if pasta.Size == 0 {
		bowl.DeletePasta(pasta.Id)
		pasta.Id = ""
		pasta.Filename = ""
		pasta.Token = ""
		pasta.ExpireDate = 0
		return pasta, nil
	}

	return pasta, nil
}

func handlerHead(w http.ResponseWriter, r *http.Request) {
	var pasta Pasta
	id := ExtractPastaId(r.URL.Path)
	if pasta, err := bowl.GetPasta(id); err != nil {
		log.Fatalf("Error getting pasta %s: %s", pasta.Id, err)
		goto ServerError
	}
	if pasta.Id == "" {
		goto NotFound
	}

	w.Header().Set("Content-Length", strconv.FormatInt(pasta.Size, 10))
	if pasta.Mime != "" {
		w.Header().Set("Content-Type", pasta.Mime)
	}
	if pasta.ExpireDate > 0 {
		w.Header().Set("Expires", time.Unix(pasta.ExpireDate, 0).Format("2006-01-02-15:04:05"))
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, "OK")
	return
ServerError:
	w.WriteHeader(500)
	fmt.Fprintf(w, "server error")
	return
NotFound:
	w.WriteHeader(404)
	fmt.Fprintf(w, "pasta not found")
	return
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	pasta, err := ReceivePasta(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "server error")
		log.Printf("Receive error: %s", err)
		return
	} else {
		if pasta.Id == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("empty pasta"))
		} else {
			log.Printf("Received bin %s (%d bytes) from %s", pasta.Id, pasta.Size, r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			url := fmt.Sprintf("%s/%s", cf.BaseUrl, pasta.Id)
			// Return format
			retFormats := r.URL.Query()["ret"]
			retFormat := ""
			if len(retFormats) > 0 {
				retFormat = retFormats[0]
			}
			if retFormat == "html" {
				// Website as return format
				fmt.Fprintf(w, "<!doctype html><html><head><title>pasta</title></head>\n")
				fmt.Fprintf(w, "<body>\n")
				fmt.Fprintf(w, "<h1>pasta</h1>\n")
				fmt.Fprintf(w, "<p>Here's your pasta: <a href=\"%s\">%s</a>", url, url)
				if pasta.ExpireDate > 0 {
					fmt.Fprintf(w, ". The pasta will expire on %s", time.Unix(pasta.ExpireDate, 0).Format("2006-01-02-15:04:05"))
				}
				fmt.Fprintf(w, "</p>\n")
				fmt.Fprintf(w, "<h3>Actions</h3>\n")
				fmt.Fprintf(w, "<p>Modification token: <code>%s</code></p>\n", pasta.Token)
				deleteLink := fmt.Sprintf("%s/delete?id=%s&token=%s", cf.BaseUrl, pasta.Id, pasta.Token)
				fmt.Fprintf(w, "<p>Accidentally uploaded a file? <a href=\"%s\">Click here to delete it</a> right away</p>\n", deleteLink)
				fmt.Fprintf(w, "</body></html>")
			} else if retFormat == "json" {
				// Dont use json package, the reply is simple enough to build it on-the-fly
				reply := fmt.Sprintf("{\"url\":\"%s\",\"token\":\"%s\"}", url, pasta.Token)
				w.Write([]byte(reply))
			} else {
				fmt.Fprintf(w, "url:   %s\ntoken: %s\n", url, pasta.Token)
			}
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Check if bin ID is given
		id := ExtractPastaId(r.URL.Path)
		if id == "" {
			handlerIndex(w, r)
		} else {
			pasta, err := bowl.GetPasta(id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Storage error")
				log.Fatalf("Storage error: %s", err)
				return
			}
			if pasta.Id == "" {
				goto NoSuchPasta
			} else {
				// Delete expired pasta if present
				if pasta.Expired() {
					if err = bowl.DeletePasta(pasta.Id); err != nil {
						log.Fatalf("Cannot deleted expired pasta %s: %s", pasta.Id, err)
					}
					goto NoSuchPasta
				}

				if err = SendPasta(pasta, w); err != nil {
					log.Printf("Error sending pasta %s: %s", pasta.Id, err)
				}
			}
		}
	} else if r.Method == http.MethodPost || r.Method == http.MethodPut {
		handlerPost(w, r)
	} else if r.Method == http.MethodDelete {
		id := ExtractPastaId(r.URL.Path)
		token := takeFirst(r.URL.Query()["token"])
		deletePasta(id, token, w)
	} else if r.Method == http.MethodHead {
		handlerHead(w, r)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unsupported method"))
	}
	return
NoSuchPasta:
	w.WriteHeader(404)
	fmt.Fprintf(w, "No pasta\n\nSorry, there is no pasta for this link")
	return
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

// Delete pasta
func handlerDelete(w http.ResponseWriter, r *http.Request) {
	id := takeFirst(r.URL.Query()["id"])
	token := takeFirst(r.URL.Query()["token"])
	deletePasta(id, token, w)
}

func handlerIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<!doctype html><html><head><title>pasta</title></head>\n")
	fmt.Fprintf(w, "<body>\n")
	fmt.Fprintf(w, "<h1>pasta</h1>\n")
	fmt.Fprintf(w, "<p>Stupid simple pastebin service</p>\n")
	fmt.Fprintf(w, "<p>Post a file: <code>curl -X POST '%s' --data-binary @FILE</code></p>\n", cf.BaseUrl)
	fmt.Fprintf(w, "<h3>File upload form</h3>\n")
	fmt.Fprintf(w, "<form enctype=\"multipart/form-data\" method=\"post\" action=\"/?ret=html\">\n")
	fmt.Fprintf(w, "<input type=\"file\" name=\"file\">\n")
	fmt.Fprintf(w, "<input type=\"submit\" value=\"Upload\">\n")
	fmt.Fprintf(w, "</form>\n")
	fmt.Fprintf(w, "</body></html>")
}

func cleanupThread() {
	for {
		duration := time.Now().Unix()
		if err := bowl.RemoveExpired(); err != nil {
			log.Fatalf("Error while removing expired pastas: %s", err)
		}
		duration = time.Now().Unix() - duration + int64(cf.CleanupInterval)
		if duration > 0 {
			time.Sleep(time.Duration(cf.CleanupInterval) * time.Second)
		}
	}
}

func main() {
	configFile := "pastad.toml"
	// Set defaults
	cf.BaseUrl = "http://localhost:8199"
	cf.PastaDir = "pastas/"
	cf.BindAddr = "127.0.0.1:8199"
	cf.MaxPastaSize = 1024 * 1024 * 25 // Default max size: 25 MB
	cf.PastaCharacters = 8             // Note: Never use less than 8 characters!
	cf.MimeTypesFile = "mime.types"
	cf.DefaultExpire = 0
	cf.CleanupInterval = 60 * 60 // Default cleanup is once per hour
	rand.Seed(time.Now().Unix())
	fmt.Println("Starting pasta server ... ")
	if FileExists(configFile) {
		if _, err := toml.DecodeFile(configFile, &cf); err != nil {
			fmt.Printf("Error loading configuration file: %s\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Config file '%s' not found\n", configFile)
	}

	// Sanity check
	if cf.PastaCharacters < 8 {
		log.Println("Warning: Using less than 8 pasta characters is not recommended and might lead to unintended side-effects")
	}
	if cf.PastaDir == "" {
		cf.PastaDir = "."
	}
	bowl.Directory = cf.PastaDir
	os.Mkdir(bowl.Directory, os.ModePerm)

	// Load MIME types file
	if cf.MimeTypesFile == "" {
		mimeExtensions = make(map[string]string, 0)
	} else {
		var err error
		mimeExtensions, err = loadMimeTypes(cf.MimeTypesFile)
		if err != nil {
			log.Printf("Warning: Cannot load mime types file '%s': %s", cf.MimeTypesFile, err)
		} else {
			log.Printf("Loaded %d mime types", len(mimeExtensions))
		}
	}

	// Start cleanup thread
	if cf.CleanupInterval > 0 {
		go cleanupThread()
	}

	// Setup webserver
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", handlerHealth)
	http.HandleFunc("/delete", handlerDelete)
	log.Printf("Serving http://%s", cf.BindAddr)
	log.Fatal(http.ListenAndServe(cf.BindAddr, nil))
}
