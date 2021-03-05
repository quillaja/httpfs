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
	"io"
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

// common http ports
const (
	httpPort  = 80
	httpsPort = 443
)

type apikey string
type directory string

// Settings for the application.
type Settings struct {
	// Port on which to listen
	Port int

	// directory for root of served filesystem
	FileRoot string

	// TLS certificate filepaths
	TLSCertPath string
	TLSKeyPath  string

	// api key -> directory map
	APIKeys map[apikey]directory
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
		Port:        httpsPort,
		FileRoot:    "files",
		TLSCertPath: "path/to/certificate",
		TLSKeyPath:  "path/to/key",
		APIKeys:     map[apikey]directory{"api_key": "dir_for_this_key"},
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
	settingsPath := flag.String("settings", "settings.json", "File containing program settings. If set to 'default', a template settings file will be written to 'default.json'.")
	flag.Parse()

	log.SetOutput(os.Stdout) // send log to stdout for systemd

	if *settingsPath == "default" {
		DefaultSettings().Save("default.json")
		log.Println("Default template settings file written to 'default.json'.")
		os.Exit(0)
	}
	s, err := OpenSettings(*settingsPath)
	if err != nil {
		log.Fatalf("error opening settings '%s': %s\n", *settingsPath, err)
	}

	http.HandleFunc("/", reqHandler(s))

	address := fmt.Sprintf(":%d", s.Port)
	log.Printf("listening for https on %s\n", address)
	err = http.ListenAndServeTLS(address, s.TLSCertPath, s.TLSKeyPath, nil)
	if err != nil {
		log.Fatalf("error starting server: %s\n", err)
	}
}

// reqHandler decodes, validates, and executes the request.
func reqHandler(s Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		// authorize
		username, key, ok := req.BasicAuth()
		userdir, found := s.APIKeys[apikey(key)]
		if !ok || !found {
			log.Printf("request with unrecognized api key '%s'\n", key)
			http.Error(w, "unrecognized api key", http.StatusUnauthorized)
			return
		}

		// get file to process
		resourcePath := req.URL.Path
		localpath := filepath.Join(s.FileRoot, string(userdir), resourcePath)
		if resourcePath == "/" {
			log.Printf("error: no file specified by '%s':'%s'\n", username, key)
			http.Error(w, "no file specified", http.StatusBadRequest)
			return
		}
		log.Printf("%s '%s' from '%s':'%s'\n", req.Method, localpath, username, key)

		// do something with file depending on http method
		var doing string
		var err error
		defer req.Body.Close()

		switch req.Method {
		case http.MethodGet:
			doing = "reading"
			err = readFile(localpath, w)

		case http.MethodDelete:
			doing = "deleting"
			err = deleteFile(localpath)

		case http.MethodPost:
			doing = "appending"
			err = writeFile(os.O_APPEND, localpath, req.Body)

		case http.MethodPut:
			doing = "truncating"
			err = writeFile(os.O_TRUNC, localpath, req.Body)

		default:
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
			return
		}

		if err != nil {
			log.Printf("error %s:%s\n", req.Method, err)
			http.Error(w, fmt.Sprintf("error %s file", doing), http.StatusInternalServerError)
		}

	}
}

// writeFile appends or truncates, according to the flag, the file at path,
// creating the file and any required directories.
func writeFile(flag int, path string, src io.Reader) error {
	// create directories if necessary
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("error creating directories '%s': %w", dir, err)
	}

	// open file
	file, err := os.OpenFile(path, flag|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", path, err)
	}
	defer file.Close()

	// write
	_, err = io.Copy(file, src)
	if err != nil {
		return fmt.Errorf("error writing payload to %s: %w", path, err)
	}

	return nil
}

// readFile reads the file at path and write its contents into dest.
func readFile(path string, dest io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", path, err)
	}
	defer file.Close()

	_, err = io.Copy(dest, file)
	if err != nil {
		return fmt.Errorf("error reading file '%s': %w", path, err)
	}

	return nil
}

// deleteFile deletes the file at path.
func deleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("error deleting file '%s': %w", path, err)
	}

	return nil
}
