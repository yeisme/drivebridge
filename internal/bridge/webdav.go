package bridge

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// WebDAVHandler translates the VFS contract into a read-only-by-default
// WebDAV surface. It never holds user keys and never exposes Agent tokens.
type WebDAVHandler struct {
	VFS   *VFS
	Token string
	Root  *os.Root // staging root for writable PUT, space local root
}

type davCollection struct{}

type davProp struct {
	DisplayName   string         `xml:"D:displayname"`
	ContentLength int64          `xml:"D:getcontentlength,omitempty"`
	ETag          string         `xml:"D:getetag,omitempty"`
	Collection    *davCollection `xml:"D:resourcetype>D:collection,omitempty"`
}

type davResponse struct {
	XMLName  xml.Name `xml:"D:response"`
	Href     string   `xml:"D:href"`
	PropStat struct {
		Prop   davProp `xml:"D:prop"`
		Status string  `xml:"D:status"`
	} `xml:"D:propstat"`
}

type davMultistatus struct {
	XMLName   xml.Name      `xml:"D:multistatus"`
	XmlnsD    string        `xml:"xmlns:D,attr"`
	Responses []davResponse `xml:"D:response"`
}

func davFail(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%q}}`, code)
}

func davStatus(err error) (int, string) {
	code := err.Error()
	ok := len(code) > 0 && len(code) <= 80
	for _, r := range code {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			ok = false
			break
		}
	}
	if !ok {
		return 400, "mount_operation_failed"
	}
	switch code {
	case "not_found", "reference_not_found", "remote_not_found":
		return 404, code
	case "path_not_allowed", "read_only", "space_not_allowed":
		return 403, code
	case "target_conflict", "name_conflict", "version_conflict", "file_version_changed", "idempotency_conflict":
		return 409, code
	}
	return 400, code
}

func (h *WebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if strings.HasPrefix(auth, "db_") {
		davFail(w, 403, "agent_token_rejected")
		return
	}
	if auth == "" || auth != h.Token {
		w.Header().Set("WWW-Authenticate", `Bearer realm="drivebridge-mount"`)
		davFail(w, 401, "authentication_required")
		return
	}
	relPath := strings.Trim(path.Clean("/"+r.URL.Path), "/")
	switch r.Method {
	case "OPTIONS":
		w.Header().Set("DAV", "1")
		w.Header().Set("Allow", "OPTIONS, PROPFIND, GET, HEAD")
		w.WriteHeader(204)
	case "PROPFIND":
		h.propfind(w, r, relPath)
	case "GET", "HEAD":
		h.get(w, r, relPath)
	case "LOCK", "UNLOCK":
		davFail(w, 405, "unsupported")
	case "MKCOL":
		h.write(w, func() error { return h.VFS.Mkdir(r.Context(), relPath) }, 201)
	case "DELETE":
		h.write(w, func() error { return h.VFS.Unlink(r.Context(), relPath) }, 204)
	case "MOVE":
		h.move(w, r, relPath)
	case "PUT":
		h.put(w, r, relPath)
	default:
		davFail(w, 405, "unsupported")
	}
}

func (h *WebDAVHandler) propfind(w http.ResponseWriter, r *http.Request, relPath string) {
	n, err := h.VFS.Lookup(r.Context(), relPath)
	if err != nil {
		status, code := davStatus(err)
		davFail(w, status, code)
		return
	}
	nodes := []Node{n}
	if n.Kind == "dir" && r.Header.Get("Depth") != "0" {
		children, err := h.VFS.Readdir(r.Context(), relPath)
		if err != nil {
			status, code := davStatus(err)
			davFail(w, status, code)
			return
		}
		nodes = append(nodes, children...)
	}
	out := davMultistatus{XmlnsD: "DAV:"}
	for _, node := range nodes {
		href := "/"
		if node.SpaceID != "" {
			href = "/" + node.SpaceID + "/"
			if node.RelPath != "" {
				href = "/" + node.SpaceID + "/" + node.RelPath
			}
		}
		resp := davResponse{Href: href}
		resp.PropStat.Prop.DisplayName = node.Name
		resp.PropStat.Status = "HTTP/1.1 200 OK"
		if node.Kind == "dir" {
			resp.PropStat.Prop.Collection = &davCollection{}
		} else {
			resp.PropStat.Prop.ContentLength = node.SizeBytes
			resp.PropStat.Prop.ETag = node.VersionID
		}
		out.Responses = append(out.Responses, resp)
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(out)
}

func (h *WebDAVHandler) get(w http.ResponseWriter, r *http.Request, relPath string) {
	body, node, err := h.VFS.Open(r.Context(), relPath, r.Header.Get("X-DriveBridge-Version"))
	if err != nil {
		status, code := davStatus(err)
		davFail(w, status, code)
		return
	}
	defer body.Close()
	seeker := &localSeeker{size: node.SizeBytes, open: func() (io.ReadCloser, error) {
		body, _, err := h.VFS.Open(r.Context(), relPath, node.VersionID)
		return body, err
	}}
	defer seeker.close()
	seeker.body = body
	w.Header().Set("ETag", node.VersionID)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, node.Name, time.Time{}, seeker)
}

func (h *WebDAVHandler) write(w http.ResponseWriter, action func() error, success int) {
	if err := action(); err != nil {
		status, code := davStatus(err)
		davFail(w, status, code)
		return
	}
	w.WriteHeader(success)
}

func (h *WebDAVHandler) move(w http.ResponseWriter, r *http.Request, relPath string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		davFail(w, 400, "invalid_request")
		return
	}
	if i := strings.Index(dest, "://"); i >= 0 {
		// Only same-server destinations are honored; foreign hosts are refused.
		if j := strings.Index(dest[i+3:], "/"); j >= 0 {
			dest = dest[i+3:][j:]
		}
	}
	dest = strings.Trim(path.Clean("/"+dest), "/")
	h.write(w, func() error { return h.VFS.Rename(r.Context(), relPath, dest) }, 201)
}

func (h *WebDAVHandler) put(w http.ResponseWriter, r *http.Request, relPath string) {
	if h.Root == nil {
		davFail(w, 403, "read_only")
		return
	}
	if r.ContentLength <= 0 {
		davFail(w, 400, "file_size_invalid")
		return
	}
	if err := h.Root.MkdirAll(".drivebridge-staging", 0700); err != nil {
		davFail(w, 500, "local_write_failed")
		return
	}
	staged := ".drivebridge-staging/mount-" + newID()
	out, err := h.Root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		davFail(w, 500, "local_write_failed")
		return
	}
	n, copyErr := io.Copy(out, io.LimitReader(r.Body, r.ContentLength+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n != r.ContentLength {
		h.Root.Remove(staged)
		davFail(w, 400, "upload_interrupted")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = "mount-" + newID()
	}
	op, err := h.VFS.Create(r.Context(), relPath, staged, key)
	if err != nil {
		h.Root.Remove(staged)
		status, code := davStatus(err)
		davFail(w, status, code)
		return
	}
	// Synchronous honesty: report the staged operation; partial-write clients
	// poll operation_get with the original idempotency key like the CLI does.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, `{"data":{"operation_id":%q,"status":%q}}`, op.ID, op.Status)
}
