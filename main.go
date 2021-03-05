// This program provides a simple server that writes a string payload to file.
// Requests are POSTed to root ("/") and the body is a json object of format:
//
// {
// 	"APIKey": "your_key",
// 	"Filename": "/rooted/filepath.txt",
// 	"Payload": "some string to append\\n"
// }
//
// Files will be created in a directory configured in settings, and each API key
// will have its own subdirectory for files.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// permissions used in creating files and directories
const (
	filePerm = 0644
	dirPerm  = 0755
)

// Settings for the application.
type Settings struct {
	Port     int
	FileRoot string
	APIKeys  map[string]string
}

// OpenSettings file at the given path.
func OpenSettings(path string) (s Settings, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}
	err = json.Unmarshal(data, &s)
	if err != nil {
		return Settings{}, err
	}
	return
}

// DefaultSettings returns a populated 'default'.
func DefaultSettings() Settings {
	return Settings{
		Port:     8080,
		FileRoot: "files",
		APIKeys:  map[string]string{},
	}
}

// Save settings to the given file.
func (s Settings) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, data, filePerm)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	settingsPath := flag.String("settings", "settings.json", "File containing program settings.")
	flag.Parse()

	s, err := OpenSettings(*settingsPath)
	if err != nil {
		log.Fatalf("error opening settings '%s': %s\n", *settingsPath, err)
	}

	http.HandleFunc("/", postHandler(s))

	address := fmt.Sprintf(":%d", s.Port)
	log.Printf("listening on %s\n", address)
	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatalf("error starting server: %s\n", err)
	}
}

// Message is the expected structure of a json object POSTed to the server root.
type Message struct {
	APIKey   string
	Filename string
	Payload  string
}

// postHandler decodes, validates, and executes the request.
func postHandler(s Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// ensure POST
		if req.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusBadRequest)
			return
		}

		// decode request body into Message
		var msg Message
		dec := json.NewDecoder(req.Body)
		err := dec.Decode(&msg)
		if err != nil {
			log.Printf("error decoding request body: %s\n", err)
			http.Error(w, "malformed request body", http.StatusBadRequest)
			return
		}
		req.Body.Close()

		// check that the request provided a valid "key"
		if app, exist := s.APIKeys[msg.APIKey]; !exist {
			log.Printf("request with unrecognized api key '%s'\n", msg.APIKey)
			http.Error(w, "unrecognized api key", http.StatusUnauthorized)
			return
		} else {
			log.Printf("processing request from '%s' given key '%s'\n", app, msg.APIKey)
		}

		// perform the actual file operation
		localpath := filepath.Join(s.FileRoot, msg.APIKey, msg.Filename)
		err = appendFile(localpath, msg.Payload)
		if err != nil {
			log.Printf("error with '%s':%s\n", localpath, err)
			http.Error(w, "error appending to file", http.StatusInternalServerError)
			return
		}
	}
}

// appendFile appends payload to the file at path, creating the file and any
// required directories.
func appendFile(path string, payload string) error {
	// create directories if necessary
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("error creating directories '%s': %w", dir, err)
	}

	// open file
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("error with requested file '%s': %w", path, err)
	}
	defer file.Close()

	// write
	_, err = file.WriteString(payload)
	if err != nil {
		return fmt.Errorf("error writing payload to %s: %w", path, err)
	}

	return nil
}
