package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
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
}

func OpenStorage(filename string) (Storage, error) {
	stor := Storage{}
	return stor, stor.Open(filename)
}

func (stor *Storage) Open(filename string) error {
	var err error
	stor.file, err = os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return err
	}
	stor.Pastas = make([]Pasta, 0)
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
		stor.Pastas = append(stor.Pastas, pasta)
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
	line := fmt.Sprintf("%s:%d:%d:%s:%s\n", pasta.Token, pasta.Date, pasta.Expire, strings.Replace(pasta.Filename, ":", "", -1), pasta.Url)
	stor.file.Write([]byte(line))
	return nil
}
