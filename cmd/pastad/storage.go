package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type Pasta struct {
	Id              string // id of the pasta
	Token           string // modification token
	DiskFilename    string // filename for the pasta on the disk
	ContentFilename string // Filename of the content
	ExpireDate      int64  // Unix() date when it will expire
	Size            int64  // file size
	Mime            string // mime type
}

func (pasta *Pasta) Expired() bool {
	if pasta.ExpireDate == 0 {
		return false
	} else {
		return time.Now().Unix() > pasta.ExpireDate
	}
}

func randBytes(n int) []byte {
	buf := make([]byte, n)
	i, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	if i < n {
		panic(fmt.Errorf("Random generator empty"))
	}
	return buf
}

func randInt8() int8 {
	buf := randBytes(1)
	return int8(buf[0])
}

func randInt() int {
	buf := randBytes(4)
	ret := 0
	for i := 0; i < 4; i++ {
		ret += int(buf[i]) << (i * 8)
	}
	return ret
}

func randByte() byte {
	buf := make([]byte, 1)
	n, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	if n < 1 {
		panic(fmt.Errorf("Random generator empty"))
	}
	return buf[0]
}

func RandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {

		b[i] = letterRunes[randInt()%len(letterRunes)]
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

/** Check for expired pastas and delete them */
func (bowl *PastaBowl) RemoveExpired() error {
	files, err := ioutil.ReadDir(bowl.Directory)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Size() == 0 {
			continue
		}
		pasta, err := bowl.GetPasta(file.Name())
		if err != nil {
			return err
		}
		if pasta.Expired() {
			if err := bowl.DeletePasta(pasta.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

// get pasta metadata
func (bowl *PastaBowl) GetPasta(id string) (Pasta, error) {
	pasta := Pasta{Id: "", DiskFilename: bowl.filename(id)}
	stat, err := os.Stat(bowl.filename(id))
	if err != nil {
		// Does not exists results in empty pasta result
		if !os.IsExist(err) {
			return pasta, nil
		}
		return pasta, err
	}
	pasta.Size = stat.Size()
	file, err := os.OpenFile(pasta.DiskFilename, os.O_RDONLY, 0400)
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
		} else if name == "mime" {
			pasta.Mime = value
		} else if name == "filename" {
			pasta.ContentFilename = value
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
	pasta.DiskFilename = bowl.filename(pasta.Id)
	file, err := os.OpenFile(pasta.DiskFilename, os.O_RDWR|os.O_CREATE, 0640)
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
	if pasta.Mime != "" {
		if _, err := file.Write([]byte(fmt.Sprintf("mime:%s\n", pasta.Mime))); err != nil {
			return err
		}
	}
	if pasta.ContentFilename != "" {
		if _, err := file.Write([]byte(fmt.Sprintf("filename:%s\n", pasta.ContentFilename))); err != nil {
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
