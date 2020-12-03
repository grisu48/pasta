/*
 * pasted - stupid simple paste server
 */

package main

import (
	"database/sql"
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
	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	BaseUrl       string `toml:"BaseURL"`  // Instance base URL
	DbFile        string `toml:"Database"` // SQLite3 database
	BinDir        string `toml:"BinsDir"`  // dir where bins are stored
	BindAddr      string `toml:"BindAddress"`
	MaxBinSize    int64  `toml:"MaxBinSize"` // Max bin size in bytes
	BinCharacters int    `toml:"BinCharacters"`
}

var cf Config

func ExtractBinName(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	} else {
		return path[i+1:]
	}
}

func SendBin(id string, w http.ResponseWriter) error {
	filename := fmt.Sprintf("%s/%s", cf.BinDir, id)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, _ := file.Stat()
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Content-Type", "text/plain")
	_, err = io.Copy(w, file)
	return err
}

func ReceiveBin(r *http.Request) (Bin, error) {
	var err error
	reader := r.Body
	buf := make([]byte, 4096)
	bin := Bin{Id: ""}
	var size int64
	defer reader.Close()

	// TODO: Use suggested ID from http header if present

	bin.Id, err = GenerateRandomBinId(cf.BinCharacters)
	if err != nil {
		log.Fatalf("Server error while generating random bin: %s", err)
		bin.Id = ""
		return bin, err
	}
	filename := fmt.Sprintf("%s/%s", cf.BinDir, bin.Id)
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return bin, err
	}
	defer file.Close()
	for size < cf.MaxBinSize {
		n, err := reader.Read(buf)
		if (err == nil || err == io.EOF) && n > 0 {
			if _, err = file.Write(buf[:n]); err != nil {
				log.Fatalf("Write error while receiving bin: %s", err)
				return bin, err
			}
			size += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("Receive error while receiving bin: %s", err)
			return bin, err
		}
	}
	if size >= cf.MaxBinSize {
		log.Println("Max size exceeded while receiving bin")
		return bin, errors.New("Bin size exceeded")
	}
	if size == 0 {
		return bin, nil
	}

	file.Sync()
	file.Close()
	bin.CreationDate = time.Now().Unix()
	bin.Size = size
	err = InsertBin(bin)
	if err != nil {
		log.Fatalf("Database while receiving bin: %s", err)
		os.Remove(filename)
		bin.Id = ""
		return bin, err
	}
	return bin, nil
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	bin, err := ReceiveBin(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Server error")
		return
	} else {
		if bin.Id == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Empty content"))
		} else {
			log.Printf("Received bin %s (%d bytes) from %s", bin.Id, bin.Size, r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			url := fmt.Sprintf("%s/%s", cf.BaseUrl, bin.Id)
			w.Write([]byte(url))
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Check if bin ID is given
		binId := ExtractBinName(r.URL.Path)
		if binId == "" {
			fmt.Fprintf(w, "<!doctype html><html><head>")
			fmt.Fprintf(w, "<body>Stupid simple pastebin service</body>")
			fmt.Fprintf(w, "</html>")
		} else {
			bin, err := FetchBin(binId)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Database error")
				log.Fatalf("Database error: %s", err)
				return
			}
			if bin.Id == "" {
				fmt.Fprintf(w, "No such bin: %s", binId)
			} else {
				if err = SendBin(bin.Id, w); err != nil {
					log.Printf("Error sending bin %s: %s", bin.Id, err)
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

func handlerPrivacy(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<!doctype html><html><head>")
	fmt.Fprintf(w, "<body><h1>Privacy</h1><p>When fetching bins no data is collected</p><p>When pasting a bin, the pasted content is stored and your IP address is logged for debugging and abuse prevention purposes</p></body>")
	fmt.Fprintf(w, "</html>")
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func main() {
	var err error
	// Set defaults
	cf.BaseUrl = "http://localhost:8199"
	cf.DbFile = "pasta.db"
	cf.BinDir = "bins/"
	cf.BindAddr = "127.0.0.1:8199"
	cf.MaxBinSize = 1024 * 1024 * 25 // Default max size: 25 MB
	cf.BinCharacters = 8             // Note: Never use less than 8 characters!
	rand.Seed(time.Now().Unix())
	if _, err := toml.DecodeFile("pastad.toml", &cf); err != nil {
		fmt.Printf("Error loading configuration file: %s\n", err)
		os.Exit(1)
	}
	if cf.BinCharacters < 8 {
		log.Println("Warning: Using less than 8 bin characters is recommended and might lead to unintended side-effects")
	}
	os.Mkdir(cf.BinDir, os.ModePerm)

	// Database
	log.Printf("Database initialization: %s", cf.DbFile)
	db, err = sql.Open("sqlite3", cf.DbFile)
	if err != nil {
		panic(err)
	}
	if err = DbInitialize(db); err != nil {
		panic(err)
	}
	defer db.Close()
	// Setup webserver
	log.Printf("Webserver initialization: http://%s", cf.BindAddr)
	http.HandleFunc("/", handler)
	http.HandleFunc("/privacy", handlerPrivacy)
	http.HandleFunc("/health", handlerHealth)
	log.Println("Startup completed")
	log.Printf("Up and running: http://%s", cf.BindAddr)
	log.Fatal(http.ListenAndServe(cf.BindAddr, nil))
}
