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

	"github.com/BurntSushi/toml"
)

type Config struct {
	RemoteHost string `toml:"RemoteHost"`
}

var cf Config

func main() {
	cf.RemoteHost = "http://localhost:8199"
	// Load configuration file if possible (swallow errors)
	homeDir, _ := os.UserHomeDir()
	configFile := homeDir + "/.pasta.toml"
	toml.DecodeFile(configFile, &cf)

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
