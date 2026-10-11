package fixtures

import (
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DiskStore retains recovery objects across preview process restarts. This
// minimal S3-shaped loopback handler is for disposable development only.
func DiskStore(dir string) (http.Handler, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Count(r.URL.Path, "/") <= 1 {
			w.WriteHeader(200)
			return
		}
		filename := filepath.Join(dir, base64.RawURLEncoding.EncodeToString([]byte(r.URL.Path)))
		switch r.Method {
		case "PUT":
			f, e := os.CreateTemp(dir, "pending-")
			if e != nil {
				http.Error(w, "write failed", 500)
				return
			}
			defer os.Remove(f.Name())
			_, e = io.Copy(f, http.MaxBytesReader(w, r.Body, (1<<30)+(16<<20)))
			if e == nil {
				e = f.Sync()
			}
			f.Close()
			if e == nil {
				e = os.Rename(f.Name(), filename)
			}
			if e != nil {
				http.Error(w, "write failed", 500)
				return
			}
			w.Header().Set("ETag", "\"fixture-etag\"")
			w.WriteHeader(200)
		case "GET", "HEAD":
			f, e := os.Open(filename)
			if e != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			st, e := f.Stat()
			if e != nil {
				http.Error(w, "read failed", 500)
				return
			}
			http.ServeContent(w, r, "object", st.ModTime(), f)
		case "DELETE":
			if e := os.Remove(filename); e != nil && !os.IsNotExist(e) {
				http.Error(w, "delete failed", 500)
				return
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}), nil
}
