package api

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web_dist
var webAssetsFS embed.FS

const webAssetsDir = "web_dist"

func init() {
	if mime.TypeByExtension(".webmanifest") == "" {
		_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	}
}

func (s *Server) handleWebAssets(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(webAssetsFS, webAssetsDir)
	if err != nil {
		http.Error(w, "asset root missing", http.StatusInternalServerError)
		return
	}

	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	reqPath = strings.TrimSuffix(reqPath, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	stat, err := fs.Stat(sub, reqPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			reqPath = "index.html"
			stat, err = fs.Stat(sub, reqPath)
			if err != nil {
				http.Error(w, "index missing", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "asset lookup failed", http.StatusInternalServerError)
			return
		}
	}
	if stat.IsDir() {
		reqPath = "index.html"
		stat, err = fs.Stat(sub, reqPath)
		if err != nil {
			http.Error(w, "index missing", http.StatusInternalServerError)
			return
		}
	}

	f, err := sub.Open(reqPath)
	if err != nil {
		http.Error(w, "asset open failed", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "asset seek unsupported", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path.Base(reqPath), stat.ModTime(), rs)
}