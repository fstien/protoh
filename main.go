// main.go
package main

import (
	"io"
	"log"
	"net/http"
	"time"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	// Prevent huge bodies from exhausting memory
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MiB
	defer r.Body.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Summary
	io.WriteString(w, "Method: "+r.Method+"\n")
	io.WriteString(w, "URL: "+r.URL.String()+"\n")
	io.WriteString(w, "Proto: "+r.Proto+"\n\n")

	// Headers
	io.WriteString(w, "Headers:\n")
	for k, vs := range r.Header {
		for _, v := range vs {
			io.WriteString(w, k+": "+v+"\n")
		}
	}
	io.WriteString(w, "\nBody:\n")

	// Echo body to response
	if _, err := io.Copy(w, r.Body); err != nil {
		http.Error(w, "error reading body: "+err.Error(), http.StatusBadRequest)
		return
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", echoHandler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("echo server listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
