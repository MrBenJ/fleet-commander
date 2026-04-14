package main

import (
	"embed"
	"io/fs"
)

//go:embed all:webdist
var webDist embed.FS

func getWebFS() (fs.FS, error) {
	return fs.Sub(webDist, "webdist")
}
