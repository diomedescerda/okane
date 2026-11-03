// Package webui serves the compiled React front end from the API binary.
//
// The deployment build copies web/dist into internal/webui/dist (see
// deploy/Dockerfile.app); only a placeholder is committed so `go build` works
// without a web build. When the placeholder is all there is the API runs in
// API-only mode and Available() returns false.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Available reports whether a real front-end build is embedded.
func Available() bool {
	_, err := embedded.ReadFile("dist/index.html")
	return err == nil
}

// Handler returns the SPA handler. Call it only when Available() is true.
// Non-asset paths (client-side routes, unknown files) fall back to index.html
// so the React router owns URL routing. Hashed Vite assets get a long-lived
// immutable cache header.
func Handler() http.Handler {
	sub, subErr := fs.Sub(embedded, "dist")
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subErr != nil || !available(sub) {
			// dist/index.html missing: front end is not built.
			http.Error(w, "front end is not built", http.StatusNotImplemented)
			return
		}
		// path.Clean over an absolute root kills traversal attempts; the
		// result is a name inside dist or an open error handled as SPA route.
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		f, err := sub.Open(name)
		if err != nil {
			serveIndex(w, sub)
			return
		}
		stat, err := f.Stat()
		_ = f.Close()
		if err != nil || stat.IsDir() {
			serveIndex(w, sub)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func available(fsys fs.FS) bool {
	_, err := fs.ReadFile(fsys, "index.html")
	return err == nil
}

func serveIndex(w http.ResponseWriter, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "front end is not built", http.StatusNotImplemented)
		return
	}
	// index.html references hashed assets, so browsers must revalidate it;
	// no-store keeps releases visible without a hard refresh.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
