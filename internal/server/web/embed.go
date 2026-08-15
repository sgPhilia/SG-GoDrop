package web

import (
	"embed"
	"io/fs"
)

var files embed.FS

func FS() fs.FS {
	return files
}
