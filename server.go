package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/quillaja/sysdlog"
)

// permissions used in creating files and directories
const (
	filePerm = 0644
	dirPerm  = 0755
)

// httpfsServer encapsulates the core functionality of the application
// around a server and logger.
type httpfsServer struct {
	settings Config
	logger   *sysdlog.LevelLogger
	server   *http.Server
}

// NewHTTPFSServer uses the Config to set up a server.
func NewHTTPFSServer(cfg Config) *httpfsServer {
	fs := &httpfsServer{
		settings: cfg,
		logger:   sysdlog.NewLevelLogger(log.New(os.Stdout, "", 0)),
	}
	fs.logger.SetLevel(sysdlog.Info) // initial level

	mux := http.NewServeMux()
	mux.HandleFunc("/", fs.reqHandler)

	fs.server = &http.Server{
		Addr:         cfg.Address,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return fs
}

// ListenAndServe begins the server
func (fs *httpfsServer) ListenAndServe() (err error) {
	fs.logger.SetLevel(sysdlog.Info)
	if fs.settings.TLSCertPath == "" || fs.settings.TLSKeyPath == "" {
		fs.logger.Println("no TLS certificate and/or key provided")
		fs.logger.Printf("listening for http on %s\n", fs.server.Addr)
		err = fs.server.ListenAndServe()
	} else {
		fs.logger.Printf("using certificate: %s, key: %s\n", fs.settings.TLSCertPath, fs.settings.TLSKeyPath)
		fs.logger.Printf("listening for https on %s\n", fs.server.Addr)
		err = fs.server.ListenAndServeTLS(fs.settings.TLSCertPath, fs.settings.TLSKeyPath)
	}
	if err != nil && err != http.ErrServerClosed {
		fs.logger.SetLevel(sysdlog.Alert)
		fs.logger.Printf("error starting server: %s\n", err)
		return err
	}
	return nil
}

// Shutdown attempts to gracefully shutdown the server.
func (fs *httpfsServer) Shutdown(ctx context.Context) error {
	fs.logger.SetLevel(sysdlog.Info)
	fs.logger.Println("attempting to shutdown server")
	return fs.server.Shutdown(ctx)
}

// reqHandler validates and executes the request.
func (fs *httpfsServer) reqHandler(w http.ResponseWriter, req *http.Request) {

	fs.logger.SetLevel(sysdlog.Info)

	// authorize
	username, key, ok := req.BasicAuth()
	userdir, found := fs.settings.APIKeys[apikey(key)]
	if !ok || !found {
		log.Printf("request with unrecognized api key '%s'\n", key)
		http.Error(w, "unrecognized api key", http.StatusUnauthorized)
		return
	}

	// get file to process
	resourcePath := req.URL.Path
	localpath := filepath.Join(fs.settings.FileRoot, string(userdir), resourcePath)
	if resourcePath == "/" {
		log.Printf("no file specified by '%s':'%s'\n", username, key)
		http.Error(w, "no file specified", http.StatusBadRequest)
		return
	}
	fs.logger.Printf("%s '%s' from '%s':'%s'\n", req.Method, localpath, username, key)

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
		fs.logger.SetLevel(sysdlog.Err)
		fs.logger.Printf("error %s:%s\n", req.Method, err)
		http.Error(w, fmt.Sprintf("error %s file", doing), http.StatusInternalServerError)
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
