package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/index.html static/style.css static/app.js static/favicon.svg
var embeddedStatic embed.FS

func StaticFS() http.FileSystem {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
