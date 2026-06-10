package embed

import (
	"embed"
	"io/fs"
)

//go:embed all:web/gocronx-admin/dist
var files embed.FS

func StaticFS() (fs.FS, error) {
	return fs.Sub(files, "web/gocronx-admin/dist")
}
