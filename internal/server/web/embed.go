package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html style.css app.js
var files embed.FS

func FS() fs.FS {
	return files
}
