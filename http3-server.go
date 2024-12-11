package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/quic-go/quic-go/http3"
)

const (
	baseDir    = "./files" // Directory to browse/upload files
	listenAddr = ":4443"
	certFile   = "secret/comodo_2024.bundle"                     // Path to your TLS certificate
	keyFile    = "secret/comodo_lausanne_ch_2024_nopassword.key" // Path to your TLS key
)

func main() {
	// Ensure the base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create base directory: %v", err))
	}

	http.HandleFunc("/", fileHandler)
	http.HandleFunc("/upload", uploadHandler)

	fmt.Printf("Starting HTTP/3 server on %s\n", listenAddr)
	err := http3.ListenAndServeTLS(listenAddr, certFile, keyFile, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}

// fileHandler serves files from the base directory
func fileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		path := filepath.Join(baseDir, r.URL.Path)
		// Prevent directory traversal
		if !strings.HasPrefix(path, filepath.Clean(baseDir)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if it's a file or a directory
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if info.IsDir() {
			// List directory contents
			files, err := os.ReadDir(path)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, "<ul>")
			for _, file := range files {
				fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, filepath.Join(r.URL.Path, file.Name()), file.Name())
			}
			fmt.Fprintln(w, "</ul>")
		} else {
			// Serve the file
			http.ServeFile(w, r, path)
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// uploadHandler handles file uploads
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form
	err := r.ParseMultipartForm(10 << 20) // Limit upload size to 10 MB
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the file
	dstPath := filepath.Join(baseDir, filepath.Base(handler.Filename))
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File %s uploaded successfully.", handler.Filename)
}
