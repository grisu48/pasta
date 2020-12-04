package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

type Pasta struct {
	Id         string
	Token      string
	Filename   string
	ExpireDate int64
	Size       int64
}

func RandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !os.IsNotExist(err)
}

/* PastaBowl is the main storage instance */
type PastaBowl struct {
	Directory string // Directory where the pastas are
}

func (bowl *PastaBowl) filename(id string) string {
	return fmt.Sprintf("%s/%s", bowl.Directory, id)
}

func (bowl *PastaBowl) Exists(id string) bool {
	return FileExists(bowl.filename(id))
}

// get pasta metadata
func (bowl *PastaBowl) GetPasta(id string) (Pasta, error) {
	pasta := Pasta{Id: "", Filename: bowl.filename(id)}
	stat, err := os.Stat(bowl.filename(id))
	if err != nil {
		// Does not exists results in empty pasta result
		if !os.IsExist(err) {
			return pasta, nil
		}
		return pasta, err
	}
	pasta.Size = stat.Size()
	file, err := os.OpenFile(pasta.Filename, os.O_RDONLY, 0400)
	if err != nil {
		return pasta, err
	}
	defer file.Close()
	// Read metadata (until "---")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return pasta, err
		}
		line := scanner.Text()
		pasta.Size -= int64(len(line) + 1)
		if line == "---" {
			break
		}
		// Parse metadata (name: value)
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		name, value := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
		if name == "token" {
			pasta.Token = value
		} else if name == "expire" {
			pasta.ExpireDate, _ = strconv.ParseInt(value, 10, 64)
		}

	}
	// All good
	pasta.Id = id
	return pasta, nil
}

func (bowl *PastaBowl) getPastaFile(id string, flag int) (*os.File, error) {
	filename := bowl.filename(id)
	file, err := os.OpenFile(filename, flag, 0640)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1)
	c := 0 // Counter
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				file.Close()
				return nil, err
			}
			file.Close()
			return nil, err
		}
		if n == 0 {
			continue
		}
		if buf[0] == '-' {
			c++
		} else if buf[0] == '\n' {
			if c >= 3 {
				return file, nil
			}
			c = 0
		}
	}
	// This should never occur
	file.Close()
	return nil, errors.New("Unexpected end of block")
}

// Get the file instance to the pasta content (read-only)
func (bowl *PastaBowl) GetPastaReader(id string) (*os.File, error) {
	return bowl.getPastaFile(id, os.O_RDONLY)
}

// Get the file instance to the pasta content (read-only)
func (bowl *PastaBowl) GetPastaWriter(id string) (*os.File, error) {
	return bowl.getPastaFile(id, os.O_RDWR)
}

// Prepare a pasta file to be written. Id and Token will be set, if not already done
func (bowl *PastaBowl) InsertPasta(pasta *Pasta) error {
	if pasta.Id == "" {
		// TODO: Use crypto rand
		pasta.Id = bowl.GenerateRandomBinId(8) // Use default length here
	}
	if pasta.Token == "" {
		// TODO: Use crypto rand
		pasta.Token = RandomString(16)
	}
	pasta.Filename = bowl.filename(pasta.Id)
	file, err := os.OpenFile(pasta.Filename, os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write([]byte(fmt.Sprintf("token:%s\n", pasta.Token))); err != nil {
		return err
	}
	if pasta.ExpireDate > 0 {
		if _, err := file.Write([]byte(fmt.Sprintf("expire:%d\n", pasta.ExpireDate))); err != nil {
			return err
		}
	}
	if _, err := file.Write([]byte("---\n")); err != nil {
		return err
	}
	return file.Sync()
}

func (bowl *PastaBowl) DeletePasta(id string) error {
	if !bowl.Exists(id) {
		return nil
	}
	return os.Remove(bowl.filename(id))
}

func (bowl *PastaBowl) GenerateRandomBinId(n int) string {
	for {
		id := RandomString(n)
		if !bowl.Exists(id) {
			return id
		}
	}
}
