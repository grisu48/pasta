/*
 * pasted - stupid simple paste server
 */

package main

import (
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
	BaseUrl       string `toml:"BaseURL"`  // Instance base URL
	PastaDir      string `toml:"PastaDir"` // dir where pasta are stored
	BindAddr      string `toml:"BindAddress"`
	MaxBinSize    int64  `toml:"MaxBinSize"` // Max bin size in bytes
	BinCharacters int    `toml:"BinCharacters"`
}

var cf Config
var bowl PastaBowl

func ExtractPastaId(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	} else {
		return path[i+1:]
	}
}

func SendPasta(id string, w http.ResponseWriter) error {
	pasta, err := bowl.GetPasta(id)
	if err != nil {
		return err
	}
	file, err := bowl.GetPastaReader(id)
	if err != nil {
		return err
	}
	defer file.Close()
	w.Header().Set("Content-Length", strconv.FormatInt(pasta.Size, 10))
	w.Header().Set("Content-Type", "text/plain")
	_, err = io.Copy(w, file)
	return err
}

func ReceivePasta(r *http.Request) (Pasta, error) {
	var err error
	reader := r.Body
	buf := make([]byte, 4096)
	pasta := Pasta{Id: ""}
	defer reader.Close()

	// TODO: Use suggested ID from http header if present

	pasta.Id = bowl.GenerateRandomBinId(cf.BinCharacters)
	if err = bowl.InsertPasta(&pasta); err != nil {
		return pasta, err
	}
	// Append contents to file
	file, err := os.OpenFile(pasta.Filename, os.O_RDWR|os.O_APPEND, 0640)
	if err != nil {
		file.Close()
		return pasta, err
	}
	defer file.Close()
	pasta.Size = 0
	for pasta.Size < cf.MaxBinSize {
		n, err := reader.Read(buf)
		if (err == nil || err == io.EOF) && n > 0 {
			if _, err = file.Write(buf[:n]); err != nil {
				log.Fatalf("Write error while receiving bin: %s", err)
				return pasta, err
			}
			pasta.Size += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("Receive error while receiving bin: %s", err)
			return pasta, err
		}
	}
	if pasta.Size >= cf.MaxBinSize {
		log.Println("Max size exceeded while receiving bin")
		return pasta, errors.New("Bin size exceeded")
	}
	if pasta.Size == 0 {
		// This is invalid
		file.Close()
		bowl.DeletePasta(pasta.Id)
		pasta.Id = ""
		pasta.Filename = ""
		pasta.Token = ""
		pasta.ExpireDate = 0
		return pasta, nil
	}

	return pasta, file.Sync()
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	pasta, err := ReceivePasta(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Server error")
		log.Printf("Receive error: %s", err)
		return
	} else {
		if pasta.Id == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Empty pasta"))
		} else {
			log.Printf("Received bin %s (%d bytes) from %s", pasta.Id, pasta.Size, r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			url := fmt.Sprintf("%s/%s", cf.BaseUrl, pasta.Id)
			// Dont use json package, the reply is simple enough to build it on-the-fly
			reply := fmt.Sprintf("{\"url\":\"%s\",\"token\":\"%s\"}", url, pasta.Token)
			w.Write([]byte(reply))
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Check if bin ID is given
		id := ExtractPastaId(r.URL.Path)
		if id == "" {
			fmt.Fprintf(w, "<!doctype html><html><head>")
			fmt.Fprintf(w, "<body>Stupid simple pastebin service</body>")
			fmt.Fprintf(w, "</html>")
		} else {
			pasta, err := bowl.GetPasta(id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Storage error")
				log.Fatalf("Storage error: %s", err)
				return
			}
			if pasta.Id == "" {
				fmt.Fprintf(w, "No such pasta: %s", id)
			} else {
				if err = SendPasta(pasta.Id, w); err != nil {
					log.Printf("Error sending pasta %s: %s", pasta.Id, err)
				}
			}
		}
	} else if r.Method == http.MethodPost || r.Method == http.MethodPut {
		handlerPost(w, r)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unsupported method"))
	}
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func main() {
	configFile := "pastad.toml"
	// Set defaults
	cf.BaseUrl = "http://localhost:8199"
	cf.PastaDir = "pastas/"
	cf.BindAddr = "127.0.0.1:8199"
	cf.MaxBinSize = 1024 * 1024 * 25 // Default max size: 25 MB
	cf.BinCharacters = 8             // Note: Never use less than 8 characters!
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
	if cf.BinCharacters < 8 {
		log.Println("Warning: Using less than 8 bin characters is recommended and might lead to unintended side-effects")
	}
	if cf.PastaDir == "" {
		cf.PastaDir = "."
	}
	bowl.Directory = cf.PastaDir
	os.Mkdir(bowl.Directory, os.ModePerm)

	// Setup webserver
	log.Printf("Webserver initialization: http://%s", cf.BindAddr)
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", handlerHealth)
	log.Printf("Startup completed. Serving http://%s", cf.BindAddr)
	log.Fatal(http.ListenAndServe(cf.BindAddr, nil))
}
