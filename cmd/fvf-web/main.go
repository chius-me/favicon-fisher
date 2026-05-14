package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chius-me/favicon-fisher/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	handler := web.NewHandler(client)
	mux := web.NewMux(handler, web.StaticFS())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("fvf-web listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
