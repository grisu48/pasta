/*
 * pasted - stupid simple paste server
 */

package main

import (
	"encoding/json"
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

const VERSION = "0.7"

var cf Config
var bowl PastaBowl
var publicPastas []Pasta
var mimeExtensions map[string]string
var delays map[string]int64
var delayMutex sync.Mutex

func SendPasta(pasta Pasta, w http.ResponseWriter) error {
	file, err := bowl.GetPastaReader(pasta.Id)
	if err != nil {
		return err
	}
	defer file.Close()
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Content-Length", strconv.FormatInt(pasta.Size, 10))
	if pasta.Mime != "" {
		w.Header().Set("Content-Type", pasta.Mime)
	}
	if pasta.ContentFilename != "" {
		w.Header().Set("Filename", pasta.ContentFilename)

	}
	_, err = io.Copy(w, file)
	return err
}

func removePublicPasta(id string) {
	copy := make([]Pasta, 0)
	for _, pasta := range publicPastas {
		if pasta.Id != id {
			copy = append(copy, pasta)
		}
	}
	publicPastas = copy
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
		// Also remove from public pastas, if present
		removePublicPasta(pasta.Id)

		w.WriteHeader(200)
		fmt.Fprintf(w, "<html><head><meta http-equiv=\"refresh\" content=\"2; url='%s'\" /></head>\n", cf.BaseUrl)
		fmt.Fprintf(w, "<body>\n")
		fmt.Fprintf(w, "<p>OK - Redirecting to <a href=\"/\">main page</a> ... </p>")
		fmt.Fprintf(w, "\n</body>\n</html>")
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

func receive(reader io.Reader, pasta *Pasta) error {
	buf := make([]byte, 4096)
	file, err := os.OpenFile(pasta.DiskFilename, os.O_RDWR|os.O_APPEND, 0640)
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

func receiveMultibody(r *http.Request, pasta *Pasta) (io.ReadCloser, bool, error) {
	public := false
	filename := ""

	// Read http headers first
	value := r.Header.Get("public")
	if value != "" {
		public = strBool(value, public)
	}
	// If the content length is given, reject immediately if the size is too big
	size := r.Header.Get("Content-Length")
	if size != "" {
		size, err := strconv.ParseInt(size, 10, 64)
		if err == nil && size > 0 && size > cf.MaxPastaSize {
			log.Println("Max size exceeded (Content-Length)")
			return nil, public, errors.New("content size exceeded")
		}
	}

	// Receive multipart form
	err := r.ParseMultipartForm(cf.MaxPastaSize)
	if err != nil {
		return nil, public, err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, public, err
	}

	// Read file headers
	filename = header.Filename
	if filename != "" {
		pasta.ContentFilename = filename
	}

	// Read form values after headers, as the form values have precedence
	form := r.MultipartForm
	values := form.Value
	if value, ok := values["public"]; ok {
		if len(value) > 0 {
			public = strBool(value[0], public)
		}
	}

	// Determine MIME type based on file extension, if present
	if filename != "" {
		pasta.Mime = mimeByFilename(filename)
	} else {
		pasta.Mime = "application/octet-stream"
	}

	return file, public, err
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

func ReceivePasta(r *http.Request) (Pasta, bool, error) {
	var err error
	var reader io.ReadCloser
	pasta := Pasta{Id: ""}
	public := false

	// Parse expire if given
	if cf.DefaultExpire > 0 {
		pasta.ExpireDate = time.Now().Unix() + cf.DefaultExpire
	}
	if expire := parseExpire(r.Header["Expire"]); expire > 0 {
		pasta.ExpireDate = expire
		// TODO: Add maximum expiration parameter
	}

	pasta.Id = removeNonAlphaNumeric(bowl.GenerateRandomBinId(cf.PastaCharacters))
	formRead := true // Read values from the form
	if isMultipart(r) {
		// InsertPasta to obtain a filename
		if err = bowl.InsertPasta(&pasta); err != nil {
			return pasta, public, err
		}
		reader, public, err = receiveMultibody(r, &pasta)
		if err != nil {
			bowl.DeletePasta(pasta.Id)
			pasta.Id = ""
			return pasta, public, err
		}
	} else {
		// Check if the input is coming from the POST form
		inputs := r.URL.Query()["input"]
		if len(inputs) > 0 && inputs[0] == "form" {
			// Copy reader, as r.FromValue consumes it's contents
			defer r.Body.Close()
			if content := r.FormValue("content"); content != "" {
				reader = io.NopCloser(strings.NewReader(content))
			} else {
				pasta.Id = "" // Empty pasta
				return pasta, public, nil
			}
		} else {
			reader = r.Body
			formRead = false
		}
	}
	defer reader.Close()

	header := r.Header
	// If the content length is given, reject immediately if the size is too big
	size := header.Get("Content-Length")
	if size != "" {
		size, err := strconv.ParseInt(size, 10, 64)
		if err == nil && size > 0 && size > cf.MaxPastaSize {
			log.Println("Max size exceeded (Content-Length)")
			return pasta, public, errors.New("content size exceeded")
		}
	}
	// Get property. URL parameter has precedence over header
	prop_get := func(name string) string {
		var val string
		if formRead {
			val = r.FormValue(name)
			if val != "" {
				return val
			}
		}
		val = header.Get(name)
		if val != "" {
			return val
		}
		return ""
	}
	// Check if public
	value := prop_get("public")
	if value != "" {
		public = strBool(value, public)
	}
	// Apply filename, if present
	// Due to inconsitent naming between URL and http parameters, we have to check for Filename and filename. URL parameters have precedence
	filename := prop_get("filename")
	if filename != "" {
		pasta.ContentFilename = filename
	} else {
		filename := prop_get("Filename")
		if filename != "" {
			pasta.ContentFilename = filename
		}
	}

	// InsertPasta sets filename
	if err = bowl.InsertPasta(&pasta); err != nil {
		return pasta, public, err
	}
	if err := receive(reader, &pasta); err != nil {
		return pasta, public, err
	}
	if pasta.Size >= cf.MaxPastaSize {
		log.Println("Max size exceeded while receiving bin")
		return pasta, public, errors.New("content size exceeded")
	}
	pasta.Mime = "text/plain"
	if pasta.Size == 0 {
		bowl.DeletePasta(pasta.Id)
		pasta.Id = ""
		pasta.DiskFilename = ""
		pasta.Token = ""
		pasta.ExpireDate = 0
		return pasta, public, nil
	}

	return pasta, public, nil
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
	id, err := ExtractPastaId(r.URL.Path)
	if err != nil {
		goto BadRequest
	}
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
BadRequest:
	w.WriteHeader(400)
	if err == nil {
		fmt.Fprintf(w, "bad request")
	} else {
		fmt.Fprintf(w, "%s", err)
	}
	return
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	delayIfRequired(r.RemoteAddr)
	pasta, public, err := ReceivePasta(r)
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
			// Save into public pastas, if this is public
			if public {
				// Store at the beginning
				pastas := make([]Pasta, 1)
				pastas[0] = pasta
				pastas = append(pastas, publicPastas...)
				publicPastas = pastas
				// Crop to maximum allowed number
				if len(publicPastas) > cf.PublicPastas {
					publicPastas = publicPastas[len(publicPastas)-cf.PublicPastas:]
				}
				if err := bowl.WritePublicPastas(publicPastas); err != nil {
					log.Printf("Error writing public pastas: %s", err)
				}
			}

			log.Printf("Received pasta %s (%d bytes) from %s", pasta.Id, pasta.Size, r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			url := fmt.Sprintf("%s/%s", cf.BaseUrl, pasta.Id)
			// Return format. URL has precedence over http heder
			retFormat := r.Header.Get("Return-Format")
			retFormats := r.URL.Query()["ret"]
			if len(retFormats) > 0 {
				retFormat = retFormats[0]
			}
			if retFormat == "html" {
				// Website as return format
				fmt.Fprintf(w, "<!doctype html><html><head><title>pasta</title></head>\n")
				fmt.Fprintf(w, "<body>\n")
				fmt.Fprintf(w, "<h1>pasta</h1>\n")
				deleteLink := fmt.Sprintf("%s/delete?id=%s&token=%s", cf.BaseUrl, pasta.Id, pasta.Token)
				fmt.Fprintf(w, "<p>Pasta: <a href=\"%s\">%s</a> | <a href=\"%s\">🗑️ Delete</a><br/>", url, url, deleteLink)
				fmt.Fprintf(w, "<pre>")
				if pasta.ContentFilename != "" {
					fmt.Fprintf(w, "Filename:           %s\n", pasta.ContentFilename)
				}
				if pasta.Mime != "" {
					fmt.Fprintf(w, "Mime-Type:          %s\n", pasta.Mime)
				}
				if pasta.Size > 0 {
					fmt.Fprintf(w, "Size:               %d B\n", pasta.Size)
				}
				if pasta.ExpireDate > 0 {
					fmt.Fprintf(w, "Expiration:         %s\n", time.Unix(pasta.ExpireDate, 0).Format("2006-01-02-15:04:05"))
				}
				if public {
					fmt.Fprintf(w, "Public:             yes\n")
				}
				fmt.Fprintf(w, "Modification token: %s\n</pre>\n", pasta.Token)
				fmt.Fprintf(w, "<p>That was fun! Fancy <a href=\"%s\">another one?</a>.</p>\n", cf.BaseUrl)
				fmt.Fprintf(w, "</body></html>")
			} else if retFormat == "json" {
				// Dont use json package, the reply is simple enough to build it on-the-fly
				reply := fmt.Sprintf("{\"url\":\"%s\",\"token\":\"%s\", \"expire\":%d}", url, pasta.Token, pasta.ExpireDate)
				w.Write([]byte(reply))
			} else {
				fmt.Fprintf(w, "url:   %s\ntoken: %s\n", url, pasta.Token)
			}
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.Method == http.MethodGet {
		// Check if bin ID is given
		id, err := ExtractPastaId(r.URL.Path)
		if err != nil {
			goto BadRequest
		}
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
		id, err := ExtractPastaId(r.URL.Path)
		if err != nil {
			goto BadRequest
		}
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
BadRequest:
	w.WriteHeader(400)
	if err == nil {
		fmt.Fprintf(w, "bad request")
	} else {
		fmt.Fprintf(w, "%s", err)
	}
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}
func handlerHealthJson(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "{\"status\":\"ok\"}")
}

func handlerPublic(w http.ResponseWriter, r *http.Request) {
	if cf.PublicPastas == 0 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "public pasta listing is disabled")
		return
	}
	w.WriteHeader(200)
	w.Write([]byte("<html>\n<head>\n<title>public pastas</title>\n</head>\n<body>"))
	w.Write([]byte("<h2>public pastas</h2>\n"))
	w.Write([]byte("<table>\n"))
	w.Write([]byte("<tr><td>Filename</td><td>Size</td></tr>\n"))
	for _, pasta := range publicPastas {
		filename := pasta.ContentFilename
		if filename == "" {
			filename = pasta.Id
		}
		w.Write([]byte(fmt.Sprintf("<tr><td><a href=\"%s\">%s</a></td><td>%d B</td></tr>\n", pasta.Id, filename, pasta.Size)))
	}
	w.Write([]byte("</table>\n"))
	fmt.Fprintf(w, "<p>The server presents at most %d public pastas.<p>\n", cf.PublicPastas)
	w.Write([]byte("</body>\n"))
}

func handlerPublicJson(w http.ResponseWriter, r *http.Request) {
	if cf.PublicPastas == 0 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "public pasta listing is disabled")
		return
	}
	type PublicPasta struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		URL      string `json:"url"`
	}
	pastas := make([]PublicPasta, 0)
	for _, pasta := range publicPastas {
		filename := pasta.ContentFilename
		if filename == "" {
			filename = pasta.Id
		}
		pastas = append(pastas, PublicPasta{Filename: filename, URL: fmt.Sprintf("%s/%s", cf.BaseUrl, pasta.Id), Size: pasta.Size})
	}
	buf, err := json.Marshal(pastas)
	if err != nil {
		log.Printf("json error (public pastas): %s\n", err)
		goto ServerError
	}
	w.WriteHeader(200)
	w.Write(buf)
	return
ServerError:
	w.WriteHeader(500)
	w.Write([]byte("Server error"))
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
	fmt.Fprintf(w, "<p>pasta is a stupid simple pastebin service for easy usage and deployment.</p>\n")
	// List public pastas, if enabled and available
	if cf.PublicPastas > 0 && len(publicPastas) > 0 {
		fmt.Fprintf(w, "<h2>Public pastas</h2>\n")
		fmt.Fprintf(w, "<table>\n")
		fmt.Fprintf(w, "<tr><td>Filename</td><td>Size</td></tr>\n")
		for _, pasta := range publicPastas {
			filename := pasta.ContentFilename
			if filename == "" {
				filename = pasta.Id
			}
			fmt.Fprintf(w, "<tr><td><a href=\"%s\">%s</a></td><td>%d B</td></tr>\n", pasta.Id, filename, pasta.Size)
		}
		fmt.Fprintf(w, "</table>\n")
		if len(publicPastas) == cf.PublicPastas {
			fmt.Fprintf(w, "<p>The server presents at most %d public pastas.<p>\n", cf.PublicPastas)
		}
	}
	fmt.Fprintf(w, "<h2>Post a new pasta</h2>\n")
	fmt.Fprintf(w, "<p><code>curl -X POST '%s' --data-binary @FILE</code></p>\n", cf.BaseUrl)
	if cf.DefaultExpire > 0 {
		fmt.Fprintf(w, "<p>pastas expire by default after %s - Enjoy them while they are fresh!</p>\n", timeHumanReadable(cf.DefaultExpire))
	}
	fmt.Fprintf(w, "<h3>File upload</h3>")
	fmt.Fprintf(w, "<p>Upload your file and make a fresh pasta out of it:</p>")
	fmt.Fprintf(w, "<form enctype=\"multipart/form-data\" method=\"post\" action=\"/?ret=html\">\n")
	fmt.Fprintf(w, "<input type=\"file\" name=\"file\">\n")
	if cf.PublicPastas > 0 {
		fmt.Fprintf(w, "<input type=\"checkbox\" id=\"public\" name=\"public\" value=\"true\"> Public\n")
	}
	fmt.Fprintf(w, "<input type=\"submit\" value=\"Upload\">\n")
	fmt.Fprintf(w, "</form>\n")
	fmt.Fprintf(w, "<h3>Text paste</h3>")
	fmt.Fprintf(w, "<p>Just paste your contents in the textfield and hit the <tt>pasta</tt> button below</p>\n")
	fmt.Fprintf(w, "<form method=\"post\" action=\"/?input=form&ret=html\">\n")
	fmt.Fprintf(w, "Filename (optional): <input type=\"text\" name=\"filename\" value=\"\" max=\"255\"><br/>\n")
	if cf.MaxPastaSize > 0 {
		fmt.Fprintf(w, "<textarea name=\"content\" rows=\"10\" cols=\"80\" maxlength=\"%d\"></textarea><br/>\n", cf.MaxPastaSize)
	} else {
		fmt.Fprintf(w, "<textarea name=\"content\" rows=\"10\" cols=\"80\"></textarea><br/>\n")
	}
	if cf.PublicPastas > 0 {
		fmt.Fprintf(w, "<input type=\"checkbox\" id=\"public\" name=\"public\" value=\"true\"> Public pasta\n")
	}
	fmt.Fprintf(w, "<input type=\"submit\" value=\"Pasta!\">\n")
	fmt.Fprintf(w, "</form>\n")
	fmt.Fprintf(w, "\n<hr/>\n")
	fmt.Fprintf(w, "<p>project page: <a href=\"https://codeberg.org/grisu48/pasta\" target=\"_BLANK\">codeberg.org/grisu48/pasta</a></p>\n")
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
	cf.SetDefaults()
	cf.ReadEnv()
	delays = make(map[string]int64)
	publicPastas = make([]Pasta, 0)
	// Parse program arguments for config
	parseCf := ParserConfig{}
	parser := argparse.NewParser("pastad", "pasta server")
	parseCf.ConfigFile = parser.String("c", "config", &argparse.Options{Default: "", Help: "Set config file"})
	parseCf.BaseURL = parser.String("B", "baseurl", &argparse.Options{Help: "Set base URL for instance"})
	parseCf.PastaDir = parser.String("d", "dir", &argparse.Options{Help: "Set pasta data directory"})
	parseCf.BindAddr = parser.String("b", "bind", &argparse.Options{Help: "Address to bind server to"})
	parseCf.MaxPastaSize = parser.Int("s", "size", &argparse.Options{Help: "Maximum allowed size for a pasta"})
	parseCf.PastaCharacters = parser.Int("n", "chars", &argparse.Options{Help: "Random characters for new pastas"})
	parseCf.MimeTypesFile = parser.String("m", "mime", &argparse.Options{Help: "Define mime types file"})
	parseCf.DefaultExpire = parser.Int("e", "expire", &argparse.Options{Help: "Pasta expire in seconds"})
	parseCf.CleanupInterval = parser.Int("C", "cleanup", &argparse.Options{Help: "Cleanup interval in seconds"})
	parseCf.PublicPastas = parser.Int("p", "public", &argparse.Options{Help: "Number of public pastas to display, if any"})
	if err := parser.Parse(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", parser.Usage(err))
		os.Exit(1)
	}
	log.Printf("Starting pasta server v%s ... \n", VERSION)
	configFile := *parseCf.ConfigFile
	if configFile != "" {
		if FileExists(configFile) {
			if _, err := toml.DecodeFile(configFile, &cf); err != nil {
				fmt.Printf("Error loading configuration file: %s\n", err)
				os.Exit(1)
			}
		} else {
			if err := CreateDefaultConfigfile(configFile); err == nil {
				fmt.Fprintf(os.Stderr, "Created default config file '%s'\n", configFile)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Cannot create default config file '%s': %s\n", configFile, err)
			}
		}
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

	// Preparation steps
	baseURL, err := ApplyMacros(cf.BaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error applying macros: %s", err)
		os.Exit(1)
	}
	cf.BaseUrl = baseURL
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

	// Load public pastas
	if cf.PublicPastas > 0 {
		pastas, err := bowl.GetPublicPastas()
		if err != nil {
			log.Printf("Error loading public pastas: %s", err)
		} else {
			// Crop if necessary
			if len(pastas) > cf.PublicPastas {
				pastas = pastas[len(pastas)-cf.PublicPastas:]
				bowl.WritePublicPastaIDs(pastas)
			}
			for _, id := range pastas {
				if id == "" {
					continue
				}
				pasta, err := bowl.GetPasta(id)
				if err == nil && pasta.Id != "" {
					publicPastas = append(publicPastas, pasta)
				}
			}
			log.Printf("Loaded %d public pastas", len(publicPastas))
		}
	}

	// Start cleanup thread
	if cf.CleanupInterval > 0 {
		go cleanupThread()
	}

	// Setup webserver
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", handlerHealth)
	http.HandleFunc("/health.json", handlerHealthJson)
	http.HandleFunc("/public", handlerPublic)
	http.HandleFunc("/public.json", handlerPublicJson)
	http.HandleFunc("/delete", handlerDelete)
	http.HandleFunc("/robots.txt", handlerRobots)
	log.Printf("Serving http://%s", cf.BindAddr)
	log.Fatal(http.ListenAndServe(cf.BindAddr, nil))
}
