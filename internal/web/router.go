package web

import (
	"net/http"
)

func NewMux(handler *Handler, staticFS http.FileSystem) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/preview", handler.Preview)
	mux.HandleFunc("/api/download", handler.Download)
	mux.Handle("/", http.FileServer(staticFS))
	return mux
}
