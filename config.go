package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// common http ports
const (
	httpPort  = 80
	httpsPort = 443
)

type apikey string
type directory string

// Config for the application.
type Config struct {
	// Address:Port on which to listen
	Address string

	// directory for root of served filesystem
	FileRoot string

	// TLS certificate filepaths
	TLSCertPath string
	TLSKeyPath  string

	// api key -> directory map
	APIKeys map[apikey]directory
}

// OpenConfig file at the given path.
func OpenConfig(path string) (s Config, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	err = json.Unmarshal(data, &s)
	if err != nil {
		return Config{}, err
	}
	return
}

// DefaultConfig returns a populated 'default'.
func DefaultConfig() Config {
	return Config{
		Address:     fmt.Sprintf(":%d", httpsPort),
		FileRoot:    "files",
		TLSCertPath: "path/to/certificate",
		TLSKeyPath:  "path/to/key",
		APIKeys:     map[apikey]directory{"api_key": "dir_for_this_key"},
	}
}

// Save Config to the given file.
func (s Config) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(path, data, filePerm)
	if err != nil {
		return err
	}
	return nil
}
