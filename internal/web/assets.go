package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var embeddedStatic embed.FS

func StaticFS() http.FileSystem {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
