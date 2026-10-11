package fixtures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// diskS3 serves the disk-backed streaming object store. It mirrors the
// in-memory handler's S3 contract (HEAD/PUT/GET/DELETE plus ListObjectsV2)
// but streams object bytes to and from files with no size cap, holding the
// runtime lock only around metadata, never across body streaming.
func (f *Runtime) diskS3() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.Failure {
			http.Error(w, "fixture storage failure", 503)
			return
		}
		switch r.Method {
		case "HEAD":
			w.Header().Set("ETag", `"fixture-etag"`)
			if strings.Count(r.URL.Path, "/") > 1 {
				f.mu.Lock()
				size, ok := f.diskSizes[r.URL.Path]
				f.mu.Unlock()
				if !ok {
					w.WriteHeader(404)
					return
				}
				w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			}
			w.WriteHeader(200)
		case "PUT":
			path := f.diskPath(r.URL.Path)
			if path == "" {
				http.Error(w, "fixture read failed", 500)
				return
			}
			if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			dst, e := os.Create(path)
			if e != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			// Stream outside the lock: a 100GiB object must not block
			// concurrent metadata or part uploads.
			n, copyErr := io.Copy(dst, r.Body)
			closeErr := dst.Close()
			if copyErr != nil || closeErr != nil {
				_ = os.Remove(path)
				http.Error(w, "fixture read failed", 500)
				return
			}
			f.mu.Lock()
			f.diskSizes[r.URL.Path] = n
			f.diskCounts[r.URL.Path] = f.diskCounts[r.URL.Path] + 1
			f.mu.Unlock()
			w.Header().Set("ETag", `"fixture-etag"`)
			w.WriteHeader(200)
		case "GET":
			if r.URL.Query().Get("list-type") == "2" {
				f.listCalls.Add(1)
				f.serveDiskList(w, r)
				return
			}
			f.mu.Lock()
			_, ok := f.diskSizes[r.URL.Path]
			f.mu.Unlock()
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			file, e := os.Open(f.diskPath(r.URL.Path))
			if e != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			defer file.Close()
			http.ServeContent(w, r, "object", time.Time{}, file)
		case "DELETE":
			f.mu.Lock()
			_, ok := f.diskSizes[r.URL.Path]
			delete(f.diskSizes, r.URL.Path)
			delete(f.diskCounts, r.URL.Path)
			f.mu.Unlock()
			if ok {
				_ = os.Remove(f.diskPath(r.URL.Path))
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	})
}

// diskPath maps one object key to a stable, flat on-disk name. Keys keep
// their leading slash shape hashed so no key can escape the store directory.
func (f *Runtime) diskPath(key string) string {
	if f.diskDir == "" || strings.Contains(key, "\x00") {
		return ""
	}
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(f.diskDir, hex.EncodeToString(digest[:]))
}

func (f *Runtime) serveDiskList(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Key          string    `xml:"Key"`
		Size         int64     `xml:"Size"`
		ETag         string    `xml:"ETag"`
		LastModified time.Time `xml:"LastModified"`
	}
	type commonPrefix struct {
		Prefix string `xml:"Prefix"`
	}
	out := struct {
		XMLName        xml.Name       `xml:"ListBucketResult"`
		Contents       []item         `xml:"Contents"`
		CommonPrefixes []commonPrefix `xml:"CommonPrefixes"`
		Truncated      bool           `xml:"IsTruncated"`
		NextToken      string         `xml:"NextContinuationToken"`
		KeyCount       int            `xml:"KeyCount"`
	}{Contents: []item{}, CommonPrefixes: []commonPrefix{}}
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	startAfter := r.URL.Query().Get("start-after")
	if token := r.URL.Query().Get("continuation-token"); token != "" && startAfter == "" {
		startAfter = token
	}
	maxKeys := 1000
	if raw := r.URL.Query().Get("max-keys"); raw != "" {
		if v, e := strconv.Atoi(raw); e == nil && v > 0 {
			maxKeys = v
		}
	}
	base := "/" + strings.Trim(r.URL.Path, "/") + "/"
	f.mu.Lock()
	keys := make([]string, 0, len(f.diskSizes))
	for key := range f.diskSizes {
		keys = append(keys, key)
	}
	sizes := make(map[string]int64, len(f.diskSizes))
	for k, v := range f.diskSizes {
		sizes[k] = v
	}
	f.mu.Unlock()
	sort.Strings(keys)
	seenPrefix := map[string]bool{}
	for _, key := range keys {
		relative := strings.TrimPrefix(key, base)
		if !strings.HasPrefix(relative, prefix) || relative == prefix {
			continue
		}
		if startAfter != "" && relative <= startAfter {
			continue
		}
		rest := strings.TrimPrefix(relative, prefix)
		if delimiter != "" {
			if idx := strings.Index(rest, delimiter); idx >= 0 {
				rolled := prefix + rest[:idx+len(delimiter)]
				if !seenPrefix[rolled] {
					seenPrefix[rolled] = true
					out.CommonPrefixes = append(out.CommonPrefixes, commonPrefix{rolled})
				}
				continue
			}
		}
		out.Contents = append(out.Contents, item{Key: relative, Size: sizes[key], ETag: "fixture-etag", LastModified: time.Now().UTC().Round(time.Second)})
	}
	sort.Slice(out.CommonPrefixes, func(i, j int) bool { return out.CommonPrefixes[i].Prefix < out.CommonPrefixes[j].Prefix })
	if len(out.Contents) > maxKeys {
		out.Truncated = true
		out.NextToken = out.Contents[maxKeys-1].Key
		out.Contents = out.Contents[:maxKeys]
	}
	out.KeyCount = len(out.Contents) + len(out.CommonPrefixes)
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(out)
}

// DiskUsageBytes reports current occupancy of the disk-backed store.
func (f *Runtime) DiskUsageBytes() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var total int64
	for _, size := range f.diskSizes {
		total += size
	}
	return total
}

// DiskPutCount reports how many PUT requests each object key received.
func (f *Runtime) DiskPutCount(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.diskCounts[key]
}

var _ = fmt.Sprintf // keep fmt for future fixtures diagnostics
