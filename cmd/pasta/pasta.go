/*
 * pasta client
 */
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"flag"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RemoteHost string `toml:"RemoteHost"`
}

var cf Config

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !os.IsNotExist(err)
}

func main() {
	cf.RemoteHost = "http://localhost:8199"
	// Load configuration file if possible (swallow errors)
	homeDir, _ := os.UserHomeDir()
	configFile := homeDir + "/.pasta.toml"
	if FileExists(configFile) {
		if _, err := toml.DecodeFile(configFile, &cf); err != nil {
			fmt.Fprintf(os.Stderr, "config-toml file parse error: %s %s\n", configFile, err)
		}
	}
	// Parse program arguments
	flag.StringVar(&cf.RemoteHost, "r", cf.RemoteHost, "Specify remote host")
    flag.Parse()

	reader := bufio.NewReader(os.Stdin)
	// Push to server
	resp, err := http.Post(cf.RemoteHost, "text/plain", reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http error: %s\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http fetch error: %s\n", err)
		os.Exit(1)
	}
	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "http status code %d\n", err)
		fmt.Println(string(body))
		os.Exit(1)
	}
	fmt.Println(string(body))
}
