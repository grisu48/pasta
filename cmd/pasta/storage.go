package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Pasta struct {
	Url      string `json:"url"`
	Token    string `json:"token"`
	Date     int64  `json:"date"`
	Expire   int64  `json:"expire"`
	Filename string `json:"filename"`
}

type Storage struct {
	Pastas   []Pasta
	file     *os.File
	filename string
	expired  int // number of expired pastas when loading
}

/* Format for writing to storage*/
func (pasta *Pasta) format() string {
	return fmt.Sprintf("%s:%d:%d:%s:%s", pasta.Token, pasta.Date, pasta.Expire, strings.Replace(pasta.Filename, ":", "", -1), pasta.Url)
}

func OpenStorage(filename string) (Storage, error) {
	stor := Storage{filename: filename}
	return stor, stor.Open(filename)
}

func (stor *Storage) Open(filename string) error {
	var err error
	stor.filename = filename
	stor.file, err = os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return err
	}
	stor.Pastas = make([]Pasta, 0)
	dirty := false // dirty flag used to rewrite the file if some pastas are expired
	stor.expired = 0
	now := time.Now().Unix()
	// Read file
	scanner := bufio.NewScanner(stor.file)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			stor.file.Close()
			stor.file = nil
			return err
		}
		split := strings.Split(scanner.Text(), ":")
		if len(split) < 5 {
			continue
		}
		pasta := Pasta{Token: split[0], Filename: split[3], Url: strings.Join(split[4:], ":")}
		pasta.Date, _ = strconv.ParseInt(split[1], 10, 64)
		pasta.Expire, _ = strconv.ParseInt(split[2], 10, 64)
		// Don't add expired pastas and mark storage as dirty for re-write in the end
		if pasta.Expire != 0 && now > pasta.Expire {
			dirty = true
			stor.expired++
		} else {
			stor.Pastas = append(stor.Pastas, pasta)
		}
	}

	// Rewrite storage if expired pastas have been removed
	if dirty {
		return stor.Write()
	}
	return nil
}

func (stor *Storage) Close() error {
	if stor.file == nil {
		return nil
	}
	return stor.file.Close()
}

func (stor *Storage) Append(pasta Pasta) error {
	if _, err := stor.file.Write([]byte(pasta.format() + "\n")); err != nil {
		return err
	}
	return stor.file.Sync()
}

/* Rewrite the whole storage file */
func (stor *Storage) Write() error {
	var err error
	stor.file.Close()
	stor.file, err = os.OpenFile(stor.filename, os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	for _, pasta := range stor.Pastas {
		if pasta.Url == "" {
			continue
		}
		_, err = stor.file.Write([]byte(pasta.format() + "\n"))
		if err != nil {
			return err
		}
	}
	return stor.file.Sync()
}

func (stor *Storage) ExpiredPastas() int {
	return stor.expired
}

func getPastaId(url string) string {
	i := strings.LastIndex(url, "/")
	if i < 0 {
		return url
	}
	return url[i+1:]
}

func (stor *Storage) Get(id string) (Pasta, bool) {
	// If the id is a url, check for url match first
	if strings.Contains(id, "://") {
		for _, pasta := range stor.Pastas {
			if pasta.Url == id {
				return pasta, true
			}
		}
	}
	// Check for pasta ID only. This needs to happen as second step als url matching has precedence
	for _, pasta := range stor.Pastas {
		if pasta.Url == id {
			return pasta, true
		}
	}

	// Nothing found, return empty pasta
	return Pasta{}, false
}

func (stor *Storage) find(url string, token string) int {
	for i, pasta := range stor.Pastas {
		if pasta.Url == url && pasta.Token == token {
			return i
		}
	}
	return -1
}

/** Marks the given pasta (given by url and token) as removed from storage. Returns true if the pasta is found, false if not found*/
func (stor *Storage) Remove(url string, token string) bool {
	i := stor.find(url, token)
	if i < 0 {
		return false
	}
	after := stor.Pastas[i+1:]
	stor.Pastas = stor.Pastas[:i]
	stor.Pastas = append(stor.Pastas, after...)
	return true

}
