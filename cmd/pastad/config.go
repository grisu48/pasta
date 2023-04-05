package main

import (
	"fmt"
	"os"
)

type Config struct {
	BaseUrl         string `toml:"BaseURL"`  // Instance base URL
	PastaDir        string `toml:"PastaDir"` // dir where pasta are stored
	BindAddr        string `toml:"BindAddress"`
	MaxPastaSize    int64  `toml:"MaxPastaSize"` // Max bin size in bytes
	PastaCharacters int    `toml:"PastaCharacters"`
	MimeTypesFile   string `toml:"MimeTypes"`    // Load mime types from this file
	DefaultExpire   int64  `toml:"Expire"`       // Default expire time for a new pasta in seconds
	CleanupInterval int    `toml:"Cleanup"`      // Seconds between cleanup cycles
	RequestDelay    int64  `toml:"RequestDelay"` // Required delay between requests in milliseconds
	PublicPastas    int    `toml:"PublicPastas"` // Number of pastas to display on public page or 0 to disable
}

type ParserConfig struct {
	ConfigFile      *string
	BaseURL         *string
	PastaDir        *string
	BindAddr        *string
	MaxPastaSize    *int // parser doesn't support int64
	PastaCharacters *int
	MimeTypesFile   *string
	DefaultExpire   *int // parser doesn't support int64
	CleanupInterval *int
	PublicPastas    *int
}

func CreateDefaultConfigfile(filename string) error {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	content := []byte(fmt.Sprintf("BaseURL = 'http://%s:8199'\nBindAddress = ':8199'\nPastaDir = 'pastas'\nMaxPastaSize = 5242880       # 5 MiB\nPastaCharacters = 8\nExpire = 2592000             # 1 month\nCleanup = 3600               # cleanup interval in seconds\nRequestDelay = 2000\nPublicPastas = 0\n", hostname))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(content); err != nil {
		return err
	}
	if err := file.Chmod(0640); err != nil {
		return err
	}
	return file.Close()
}

// SetDefaults sets the default values to a config instance
func (cf *Config) SetDefaults() {
	cf.BaseUrl = "http://localhost:8199"
	cf.PastaDir = "pastas/"
	cf.BindAddr = "127.0.0.1:8199"
	cf.MaxPastaSize = 1024 * 1024 * 25 // Default max size: 25 MB
	cf.PastaCharacters = 8             // Note: Never use less than 8 characters!
	cf.MimeTypesFile = "mime.types"
	cf.DefaultExpire = 0
	cf.CleanupInterval = 60 * 60 // Default cleanup is once per hour
	cf.RequestDelay = 0          // By default not spam protection (Assume we are in safe environment)
	cf.PublicPastas = 0
}

// ReadEnv reads the environmental variables and sets the config accordingly
func (cf *Config) ReadEnv() {
	cf.BaseUrl = getenv("PASTA_BASEURL", cf.BaseUrl)
	cf.PastaDir = getenv("PASTA_PASTADIR", cf.PastaDir)
	cf.BindAddr = getenv("PASTA_BINDADDR", cf.BindAddr)
	cf.MaxPastaSize = getenv_i64("PASTA_MAXSIZE", cf.MaxPastaSize)
	cf.PastaCharacters = getenv_i("PASTA_CHARACTERS", cf.PastaCharacters)
	cf.MimeTypesFile = getenv("PASTA_MIMEFILE", cf.MimeTypesFile)
	cf.DefaultExpire = getenv_i64("PASTA_EXPIRE", cf.DefaultExpire)
	cf.CleanupInterval = getenv_i("PASTA_CLEANUP", cf.CleanupInterval)
	cf.RequestDelay = getenv_i64("PASTA_REQUESTDELAY", cf.RequestDelay)
	cf.PublicPastas = getenv_i("PASTA_PUBLICPASTAS", cf.PublicPastas)
}

func (pc *ParserConfig) ApplyTo(cf *Config) {
	if pc.BaseURL != nil && *pc.BaseURL != "" {
		cf.BaseUrl = *pc.BaseURL
	}
	if pc.PastaDir != nil && *pc.PastaDir != "" {
		cf.PastaDir = *pc.PastaDir
	}
	if pc.BindAddr != nil && *pc.BindAddr != "" {
		cf.BindAddr = *pc.BindAddr
	}
	if pc.MaxPastaSize != nil && *pc.MaxPastaSize > 0 {
		cf.MaxPastaSize = int64(*pc.MaxPastaSize)
	}
	if pc.PastaCharacters != nil && *pc.PastaCharacters > 0 {
		cf.PastaCharacters = *pc.PastaCharacters
	}
	if pc.MimeTypesFile != nil && *pc.MimeTypesFile != "" {
		cf.MimeTypesFile = *pc.MimeTypesFile
	}
	if pc.DefaultExpire != nil && *pc.DefaultExpire > 0 {
		cf.DefaultExpire = int64(*pc.DefaultExpire)
	}
	if pc.CleanupInterval != nil && *pc.CleanupInterval > 0 {
		cf.CleanupInterval = *pc.CleanupInterval
	}
	if pc.PublicPastas != nil && *pc.PublicPastas > 0 {
		cf.PublicPastas = *pc.PublicPastas
	}
}
