package storage

import (
	"embed"
	"encoding/json"
	"io/fs"
)

// FS exposes the package's fs value.
//
//go:embed views ui/dist static
var FS embed.FS

var (
	// UI exposes the package's ui value.
	UI = mustSub(FS, "ui/dist")
	// Assets exposes the package's assets value.
	Assets = mustSub(UI, "assets")
	// Static exposes the package's static value.
	Static = mustSub(FS, "static")
)

func mustSub(source fs.FS, directory string) fs.FS {
	subtree, err := fs.Sub(source, directory)
	if err != nil {
		panic(err)
	}
	return subtree
}

// ManifestNode represents manifest node data.
type ManifestNode struct {
	Src     string   `json:"src"`
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
	IsEntry bool     `json:"isEntry"`
}

// Manifest represents manifest data.
type Manifest map[string]*ManifestNode

// ParseManifest parses manifest.
func ParseManifest() Manifest {
	file, err := FS.ReadFile("ui/dist/manifest.json")
	if err != nil {
		return nil
	}
	node := make(Manifest)
	err = json.Unmarshal(file, &node)
	if err != nil {
		return nil
	}
	return node
}

// GetCSSFiles returns css files.
func (m Manifest) GetCSSFiles(entry string) []string {
	node, ok := m[entry]
	if !ok {
		return []string{}
	}
	return node.CSS
}

// GetJsFiles returns js files.
func (m Manifest) GetJsFiles(entry string) []string {
	node, ok := m[entry]
	if !ok {
		return []string{}
	}
	files := make([]string, 0)
	files = append(files, node.File)
	if len(node.Imports) > 0 {
		for _, v := range node.Imports {
			inode, exist := m[v]
			if exist {
				files = append(files, inode.File)
			}
		}
	}
	return files
}
