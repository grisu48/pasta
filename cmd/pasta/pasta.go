/*
 * pasta client
 */
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RemoteHost  string       `toml:"RemoteHost"`
	RemoteHosts []RemoteHost `toml:"Remote"`
}
type RemoteHost struct {
	URL     string   `toml:"url"`     // URL of the remote host
	Alias   string   `toml:"alias"`   // Alias for the remote host
	Aliases []string `toml:"aliases"` // List of additional aliases for the remote host
}

var cf Config

// Search for the given remote alias. Returns true and the remote if found, otherwise false and an empty instance
func (cf *Config) FindRemoteAlias(remote string) (bool, RemoteHost) {
	for _, remote := range cf.RemoteHosts {
		if cf.RemoteHost == remote.Alias {
			return true, remote
		}
		for _, alias := range remote.Aliases {
			if cf.RemoteHost == alias {
				return true, remote
			}
		}
	}
	var ret RemoteHost
	return false, ret
}

/* http error instance */
type HttpError struct {
	err        string
	StatusCode int
}

func (e *HttpError) Error() string {
	return e.err
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !os.IsNotExist(err)
}

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] [FILE,[FILE2,...]]\n\n", os.Args[0])
	fmt.Println("OPTIONS")
	fmt.Println("     -h, --help                 Print this help message")
	fmt.Println("     -r, --remote HOST          Define remote host or alias (Default: http://localhost:8199)")
	fmt.Println("     -c, --config FILE          Define config file (Default: ~/.pasta.toml)")
	fmt.Println("     -f, --file FILE            Send FILE to server")
	fmt.Println("")
	fmt.Println("     --ls, --list               List known pasta pushes")
	fmt.Println("     --gc                       Garbage collector (clean expired pastas)")
	fmt.Println("")
	fmt.Println("One or more files can be fined which will be pushed to the given server")
	fmt.Println("If no file is given, the input from stdin will be pushed")
}

func push(src io.Reader) (Pasta, error) {
	pasta := Pasta{}
	resp, err := http.Post(cf.RemoteHost+"?ret=json", "text/plain", src)
	if err != nil {
		return pasta, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return pasta, fmt.Errorf("http status code: %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&pasta)
	if err != nil {
		return pasta, err
	}
	return pasta, nil
}

func httpRequest(url string, method string) error {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	} else {
		// Try to fetch a small error message
		buf := make([]byte, 200)
		n, err := resp.Body.Read(buf)
		if err != nil || n == 0 || n >= 200 {
			return &HttpError{err: fmt.Sprintf("http code %d", resp.StatusCode), StatusCode: resp.StatusCode}
		}
		return &HttpError{err: fmt.Sprintf("http code %d: %s", resp.StatusCode, string(buf)), StatusCode: resp.StatusCode}
	}
}

func rm(pasta Pasta) error {
	url := fmt.Sprintf("%s?token=%s", pasta.Url, pasta.Token)
	if err := httpRequest(url, "DELETE"); err != nil {
		// Ignore 404 errors, because that means that the pasta is remove on the server (e.g. expired)
		if strings.HasPrefix(err.Error(), "http code 404") {
			return nil
		}
		return err
	}
	return nil
}

func getFilename(filename string) string {
	i := strings.LastIndex(filename, "/")
	if i < 0 {
		return filename
	} else {
		return filename[i+1:]
	}
}

func main() {
	cf.RemoteHost = "http://localhost:8199"
	action := ""
	// Load configuration file if possible
	homeDir, _ := os.UserHomeDir()
	configFile := homeDir + "/.pasta.toml"
	if FileExists(configFile) {
		if _, err := toml.DecodeFile(configFile, &cf); err != nil {
			fmt.Fprintf(os.Stderr, "config-toml file parse error: %s %s\n", configFile, err)
		}
	}
	// Files to be pushed
	files := make([]string, 0)
	explicit := false // marking files as explicitly given. This disabled the shortcut commands (ls, rm, gc)
	// Parse program arguments
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}
		if arg[0] == '-' {
			if arg == "-h" || arg == "--help" {
				usage()
				os.Exit(0)
			} else if arg == "-r" || arg == "--remote" {
				i++
				cf.RemoteHost = args[i]
			} else if arg == "-c" || arg == "--config" {
				i++
				if _, err := toml.DecodeFile(args[i], &cf); err != nil {
					fmt.Fprintf(os.Stderr, "config-toml file parse error: %s %s\n", configFile, err)
				}
			} else if arg == "-f" || arg == "--file" {
				i++
				explicit = true
				files = append(files, args[i])
			} else if arg == "--ls" || arg == "--list" {
				action = "list"
			} else if arg == "--rm" || arg == "--remote" || arg == "--delete" {
				action = "rm"
			} else if arg == "--gc" {
				action = "gc"
			} else {
				fmt.Fprintf(os.Stderr, "Invalid argument: %s\n", arg)
				os.Exit(1)
			}
		} else {
			files = append(files, arg)
		}
	}
	if found, remote := cf.FindRemoteAlias(cf.RemoteHost); found {
		fmt.Fprintf(os.Stderr, "Alias found: %s for %s\n", cf.RemoteHost, remote.URL)
		cf.RemoteHost = remote.URL
	}
	// Sanity checks
	if strings.Index(cf.RemoteHost, "://") < 0 {
		fmt.Fprintf(os.Stderr, "Invalid remote: %s\n", cf.RemoteHost)
		os.Exit(1)
	}
	// Load stored pastas
	stor, err := OpenStorage(homeDir + "/.pastas.dat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open pasta storage: %s\n", err)
	}

	if !explicit {
		// Special action: "pasta ls" list pasta
		if action == "" && len(files) == 1 && files[0] == "ls" {
			if FileExists(files[0]) {
				fmt.Fprintf(os.Stderr, "Ambiguous command %s (file '%s' exists) - please use '-f %s' to upload or --ls to list pastas\n", files[0], files[0], files[0])
				os.Exit(1)
			}
			action = "list"
			files = make([]string, 0)
		}
		// Special action: "pasta rm" is the same as "pasta --rm"
		if len(files) > 1 && files[0] == "rm" {
			if FileExists(files[0]) {
				fmt.Fprintf(os.Stderr, "Ambiguous command %s (file '%s' exists) - please use '-f %s' to upload or --rm to remove pastas\n", files[0], files[0], files[0])
				os.Exit(1)
			}
			action = "rm"
			files = files[1:]
		}
		// Special action: "pasta gc" is the same as "pasta --gc"
		if len(files) == 1 && (files[0] == "gc" || files[0] == "clean" || files[0] == "expire") {
			if FileExists(files[0]) {
				fmt.Fprintf(os.Stderr, "Ambiguous command %s (file '%s' exists) - please use '-f %s' to upload or --gc to cleanup expired pastas\n", files[0], files[0], files[0])
				os.Exit(1)
			}
			action = "gc"
			files = files[1:]
		}
	}

	if action == "push" || action == "" {
		if len(files) > 0 {
			for _, filename := range files {
				file, err := os.OpenFile(filename, os.O_RDONLY, 0400)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", filename, err)
					os.Exit(1)
				}
				defer file.Close()
				if stat, err := file.Stat(); err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", filename, err)
					os.Exit(1)
				} else if stat.Size() == 0 {
					fmt.Fprintf(os.Stderr, "Skipping empty file %s\n", filename)
					continue
				}
				// Push file
				pasta, err := push(file)
				pasta.Filename = getFilename(filename)
				pasta.Date = time.Now().Unix()
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					os.Exit(1)
				}
				if err = stor.Append(pasta); err != nil {
					fmt.Fprintf(os.Stderr, "Cannot writing pasta to local store: %s\n", err)
				}
				// For a single file just print the link
				if len(files) == 1 {
					fmt.Printf("%s\n", pasta.Url)
				} else {
					fmt.Printf("%s - %s\n", pasta.Filename, pasta.Url)
				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "Reading from stdin")
			reader := bufio.NewReader(os.Stdin)
			pasta, err := push(reader)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				os.Exit(1)
			}
			if err = stor.Append(pasta); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot writing pasta to local store: %s\n", err)
			}
			fmt.Println(pasta.Url)
		}
	} else if action == "list" { // list known pastas
		if len(stor.Pastas) > 0 {
			fmt.Printf("Id   %-30s   %-19s   %s\n", "Filename", "Date", "URL")
			for i, pasta := range stor.Pastas {
				t := time.Unix(pasta.Date, 0)
				filename := pasta.Filename
				if filename == "" {
					filename = "<none>"
				}
				fmt.Printf("%-3d  %-30s   %-19s   %s\n", i, filename, t.Format("2006-01-02 15:04:05"), pasta.Url)
			}
		}
	} else if action == "rm" { // remove pastas
		// List of pastas to be deleted
		spoiled := make([]Pasta, 0)
		// Match given pastas and get tokens
		for _, file := range files {
			// If it is and integer, take the n-th item
			if id, err := strconv.Atoi(file); err == nil {
				if id < 0 || id >= len(stor.Pastas) {
					fmt.Fprintf(os.Stderr, "Cannot find pasta '%d'\n", id)
					os.Exit(1)
				}
				spoiled = append(spoiled, stor.Pastas[id])
			} else {
				if pasta, ok := stor.Get(file); ok {
					spoiled = append(spoiled, pasta)
				} else {
					// Stop execution
					fmt.Fprintf(os.Stderr, "Cannot find pasta '%s'\n", file)
					os.Exit(1)
				}
			}
		}

		// Delete found pastas
		for _, pasta := range spoiled {
			if err := rm(pasta); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting '%s': %s\n", pasta.Url, err)
			} else {
				fmt.Printf("Deleted: %s\n", pasta.Url)
				stor.Remove(pasta.Url, pasta.Token) // Mark as removed for when rewriting storage
			}
		}
		// And re-write storage
		if err = stor.Write(); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to local storage: %s\n", err)
		}
	} else if action == "gc" || action == "clean" {
		// Cleanup happens when loading pastas
		expired := stor.ExpiredPastas()
		if expired == 0 {
			fmt.Println("all good")
		} else if expired == 1 {
			fmt.Println("one expired pasta cleared")
		} else {
			fmt.Printf("%d expired pastas cleared\n", expired)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Unkown action: %s\n", action)
		os.Exit(1)
	}
}
