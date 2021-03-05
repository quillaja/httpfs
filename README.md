## httpfs

This program provides a simple server that provides basic CRUD access to
a filesystem on the server. Files are specified using the URL
(eg www.example.com/mypath/myfile.txt) and the action is specified using
HTTP methods:

	GET - read the entire file
	POST - create/append to the file
	PUT - create/truncate (overwrite) the file
	DELETE - delete the file

Authorization credentials are provided via the `Authorization` HTTP header,
using the `Basic` scheme. Instead of a "password", a previously obtained API
key is used. A username should be provided but is not currently used. The server
should use HTTPS to encrypt the credentials and file contents over the wire.
Files will be created in a directory configured in settings, and each API key
will have its own subdirectory for files.
A settings file must be provided. `APIKeys` maps api keys to their "sandbox"
subdirectory of the `FileRoot`. API keys must be unique, but multiple keys
can map to the same subdirectory.
For example:
	
    {
	  "Port": 443,
	  "FileRoot": "files",
	  "TLSCertPath": "path/to/certificate",
	  "TLSKeyPath": "path/to/key",
	  "APIKeys": {
	    "SOME_KEY_1234": "hamburger",
	    "ANOTHER_KEY_0987": "hotdog"
	  }
	}
