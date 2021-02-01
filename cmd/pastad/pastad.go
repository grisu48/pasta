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
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/akamensky/argparse"
)

type Config struct {
	BaseUrl         string `toml:"BaseURL"`  // Instance base URL
	PastaDir        string `toml:"PastaDir"` // dir where pasta are stored
	BindAddr        string `toml:"BindAddress"`
	MaxPastaSize    int64  `toml:"MaxPastaSize"` // Max bin size in bytes
	PastaCharacters int    `toml:"PastaCharacters"`
	MimeTypesFile   string `toml:"MimeTypes`     // Load mime types from this file
	DefaultExpire   int64  `toml:"Expire"`       // Default expire time for a new pasta in seconds
	CleanupInterval int    `toml:"Cleanup"`      // Seconds between cleanup cycles
	RequestDelay    int64  `toml:"RequestDelay"` // Required delay between requests in milliseconds
}

type ParserConfig struct {
	ConfigFile      *string
	BaseURL         *string
	PastaDir        *string
	BindAddr        *string
	MaxPastaSize    *int // parser doesn't support int64
	PastaCharacters *int
	MimeTypesFile   *string
	DefaultExpire   *int // parser doesn't support int64
	CleanupInterval *int
}

var cf Config
var bowl PastaBowl
var mimeExtensions map[string]string

func (pc *ParserConfig) ApplyTo(cf *Config) {
	if pc.BaseURL != nil && *pc.BaseURL != "" {
		cf.BaseUrl = *pc.BaseURL
	}
	if pc.PastaDir != nil && *pc.PastaDir != "" {
		cf.PastaDir = *pc.PastaDir
	}
	if pc.BindAddr != nil && *pc.BindAddr != "" {
		cf.BindAddr = *pc.BindAddr
	}
	if pc.MaxPastaSize != nil && *pc.MaxPastaSize > 0 {
		cf.MaxPastaSize = int64(*pc.MaxPastaSize)
	}
	if pc.PastaCharacters != nil && *pc.PastaCharacters > 0 {
		cf.PastaCharacters = *pc.PastaCharacters
	}
	if pc.MimeTypesFile != nil && *pc.MimeTypesFile != "" {
		cf.MimeTypesFile = *pc.MimeTypesFile
	}
	if pc.DefaultExpire != nil && *pc.DefaultExpire > 0 {
		cf.DefaultExpire = int64(*pc.DefaultExpire)
	}
	if pc.CleanupInterval != nil && *pc.CleanupInterval > 0 {
		cf.CleanupInterval = *pc.CleanupInterval
	}
}

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

var delays map[string]int64
var delayMutex sync.Mutex

/* Extract the remote IP address of the given remote
 * The remote is expected to come from http.Request and contain the IP address plus the port */
func extractRemoteIP(remote string) string {
	// Check if IPv6
	i := strings.Index(remote, "[")
	if i >= 0 {
		j := strings.Index(remote, "]")
		if j <= i {
			return remote
		}
		return remote[i+1 : j]
	}
	i = strings.Index(remote, ":")
	if i > 0 {
		return remote[:i]
	}
	return remote
}

/* Delay a request for the given remote if required by spam protection */
func delayIfRequired(remote string) {
	if cf.RequestDelay == 0 {
		return
	}
	address := extractRemoteIP(remote)
	now := time.Now().UnixNano() / 1000000 // Timestamp now in milliseconds. This should be fine until 2262
	delayMutex.Lock()
	delay, ok := delays[address]
	delayMutex.Unlock()
	if ok {
		delta := cf.RequestDelay - (now - delay)
		if delta > 0 {
			time.Sleep(time.Duration(delta) * time.Millisecond)
		}
	}
	delays[address] = time.Now().UnixNano() / 1000000 // Fresh timestamp
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
	delayIfRequired(r.RemoteAddr)
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
				fmt.Fprintf(w, "<p><a href=\"%s\">%s</a>", url, url)
				if pasta.ExpireDate > 0 {
					fmt.Fprintf(w, " (expires %s)", time.Unix(pasta.ExpireDate, 0).Format("2006-01-02-15:04:05"))
				}
				fmt.Fprintf(w, "</p>\n")
				deleteLink := fmt.Sprintf("%s/delete?id=%s&token=%s", cf.BaseUrl, pasta.Id, pasta.Token)
				fmt.Fprintf(w, "<p>Accidentally uploaded? <a href=\"%s\">Delete it</a> right away</p>\n", deleteLink)
				fmt.Fprintf(w, "<p>Modification token: <code>%s</code></p>\n", pasta.Token)
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
		delayIfRequired(r.RemoteAddr)
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

func handlerRobots(w http.ResponseWriter, r *http.Request) {
	// no robots allowed here
	fmt.Fprintf(w, "User-agent: *\nDisallow: /\n")
}

// Delete pasta
func handlerDelete(w http.ResponseWriter, r *http.Request) {
	delayIfRequired(r.RemoteAddr)
	id := takeFirst(r.URL.Query()["id"])
	token := takeFirst(r.URL.Query()["token"])
	deletePasta(id, token, w)
}

func handlerIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<!doctype html><html><head><title>pasta</title></head>\n")
	fmt.Fprintf(w, "<body>\n")
	fmt.Fprintf(w, "<h1>pasta</h1>\n")
	fmt.Fprintf(w, "<p>Stupid simple pastebin service written in go. Visit the project repo on <a href=\"https://github.com/grisu48/pasta\" target=\"_BLANK\">Github</a>.</p>\n")
	fmt.Fprintf(w, "<h3>Post a file</h3>\n")
	fmt.Fprintf(w, "<p>Just shove your file via POST request to the main page :</p>")
	fmt.Fprintf(w, "<pre>curl -X POST '%s' --data-binary @FILE</pre>\n", cf.BaseUrl)
	fmt.Fprintf(w, "<p>Or use the following upload form</p>")
	fmt.Fprintf(w, "<form enctype=\"multipart/form-data\" method=\"post\" action=\"/?ret=html\">\n")
	fmt.Fprintf(w, "<input type=\"file\" name=\"file\">\n")
	fmt.Fprintf(w, "<input type=\"submit\" value=\"Upload\">\n")
	fmt.Fprintf(w, "</form>\n")
	fmt.Fprintf(w, "</body></html>")
}

func cleanupThread() {
	// Double check this, because I know that I will screw this up at some point in the main routine :-)
	if cf.CleanupInterval == 0 {
		return
	}
	for {
		duration := time.Now().Unix()
		if err := bowl.RemoveExpired(); err != nil {
			log.Fatalf("Error while removing expired pastas: %s", err)
		}
		if cf.RequestDelay > 0 { // Cleanup of the spam protection addresses only if enabled
			delayMutex.Lock()
			delays = make(map[string]int64)
			delayMutex.Unlock()
		}
		duration = time.Now().Unix() - duration + int64(cf.CleanupInterval)
		if duration > 0 {
			time.Sleep(time.Duration(cf.CleanupInterval) * time.Second)
		} else {
			// Don't spam the system, give it at least some time
			time.Sleep(time.Second)
		}
	}
}

func main() {
	// Set defaults
	cf.BaseUrl = "http://localhost:8199"
	cf.PastaDir = "pastas/"
	cf.BindAddr = "127.0.0.1:8199"
	cf.MaxPastaSize = 1024 * 1024 * 25 // Default max size: 25 MB
	cf.PastaCharacters = 8             // Note: Never use less than 8 characters!
	cf.MimeTypesFile = "mime.types"
	cf.DefaultExpire = 0
	cf.CleanupInterval = 60 * 60 // Default cleanup is once per hour
	cf.RequestDelay = 0          // By default not spam protection (Assume we are in safe environment)
	delays = make(map[string]int64)
	// Parse program arguments for config
	parseCf := ParserConfig{}
	parser := argparse.NewParser("pastad", "pasta server")
	parseCf.ConfigFile = parser.String("c", "config", &argparse.Options{Default: "pastad.toml", Help: "Set config file"})
	parseCf.BaseURL = parser.String("B", "baseurl", &argparse.Options{Help: "Set base URL for instance"})
	parseCf.PastaDir = parser.String("d", "dir", &argparse.Options{Help: "Set pasta data directory"})
	parseCf.BindAddr = parser.String("b", "bind", &argparse.Options{Help: "Address to bind server to"})
	parseCf.MaxPastaSize = parser.Int("s", "size", &argparse.Options{Help: "Maximum allowed size for a pasta"})
	parseCf.PastaCharacters = parser.Int("n", "chars", &argparse.Options{Help: "Random characters for new pastas"})
	parseCf.MimeTypesFile = parser.String("m", "mime", &argparse.Options{Help: "Define mime types file"})
	parseCf.DefaultExpire = parser.Int("e", "expire", &argparse.Options{Help: "Pasta expire in seconds"})
	parseCf.CleanupInterval = parser.Int("C", "cleanup", &argparse.Options{Help: "Cleanup interval in seconds"})
	if err := parser.Parse(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", parser.Usage(err))
		os.Exit(1)
	}
	log.Println("Starting pasta server ... ")
	configFile := *parseCf.ConfigFile
	if configFile != "" && FileExists(configFile) {
		if _, err := toml.DecodeFile(configFile, &cf); err != nil {
			fmt.Printf("Error loading configuration file: %s\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Config file '%s' not found\n", configFile)
	}
	// Program arguments overwrite config file
	parseCf.ApplyTo(&cf)

	// Sanity check
	if cf.PastaCharacters <= 0 {
		log.Println("Setting pasta characters to default 8 because it was <= 0")
		cf.PastaCharacters = 8
	}
	if cf.PastaCharacters < 8 {
		log.Println("Warning: Using less than 8 pasta characters might not be side-effects free")
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
	http.HandleFunc("/robots.txt", handlerRobots)
	log.Printf("Serving http://%s", cf.BindAddr)
	log.Fatal(http.ListenAndServe(cf.BindAddr, nil))
}
