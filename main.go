// This program provides a simple server that provides basic CRUD access to
// a filesystem on the server. Files are specified using the URL
// (eg www.example.com/mypath/myfile.txt) and the action is specified using
// HTTP methods:
//
// 		GET - read the entire file
//		POST - create/append to the file
//		PUT - create/truncate (overwrite) the file
//		DELETE - delete the file
//
// Authorization credentials are provided via the `Authorization` HTTP header,
// using the `Basic` scheme. Instead of a "password", a previously obtained API
// key is used. A username should be provided but is not currently used. The server
// should use HTTPS to encrypt the credentials and file contents over the wire.
//
// Files will be created in a directory configured in settings, and each API key
// will have its own subdirectory for files.
//
// A settings file must be provided. `APIKeys` maps api keys to their "sandbox"
// subdirectory of the `FileRoot`. API keys must be unique, but multiple keys
// can map to the same subdirectory.
// For example:
//
//		{
//		  "Address": ":443",
//		  "FileRoot": "files",
//		  "TLSCertPath": "path/to/certificate",
//		  "TLSKeyPath": "path/to/key",
//		  "APIKeys": {
//		    "SOME_KEY_1234": "hamburger",
//		    "ANOTHER_KEY_0987": "hotdog"
//		  }
//		}
//
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("cfg", "config.json",
		"File containing program settings. If set to 'default', a template config file will be written to 'default.json'.")
	flag.Parse()

	if *configPath == "default" {
		DefaultConfig().Save("default.json")
		fmt.Println("Default template config file written to 'default.json'.")
		os.Exit(0)
	}
	cfg, err := OpenConfig(*configPath)
	if err != nil {
		fmt.Printf("fatal error opening config '%s': %s\n", *configPath, err)
		os.Exit(1)
	}

	fs := NewHTTPFSServer(cfg)
	go func() {
		if fs.ListenAndServe() != nil {
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	fs.Shutdown(ctx)
}
