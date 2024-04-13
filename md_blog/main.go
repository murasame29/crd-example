package main

import (
	"log"
	"net/http"
)

const (
	mdPath = "./blog/%s.md"
)

func main() {
	mux := http.NewServeMux()
	// service static html file
	mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		http.ServeFile(w, r, "html/"+id+".html")
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
