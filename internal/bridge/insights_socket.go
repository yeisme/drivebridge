package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"time"
)

type ScanRequest struct {
	Space         string `json:"space_id"`
	Enabled       *bool  `json:"enabled"`
	MaxEntries    int    `json:"max_entries"`
	MaxSeconds    int    `json:"max_seconds"`
	CancelID      string `json:"cancel_id"`
	Resume        string `json:"resume_id"`
	ConfigureOnly bool   `json:"configure_only"`
}

func (s *Service) RunScanRequest(ctx context.Context, q ScanRequest) (any, error) {
	if q.CancelID != "" {
		if e := s.Insights.CancelScan(localOwner, q.Space, q.CancelID); e != nil {
			return nil, e
		}
		return map[string]bool{"cancel_requested": true}, nil
	}
	if q.Enabled != nil {
		p, e := s.ConfigureScan(q.Space, *q.Enabled, q.MaxEntries, q.MaxSeconds)
		if e != nil {
			return nil, e
		}
		if !*q.Enabled || q.ConfigureOnly {
			return p, nil
		}
	}
	return s.Scan(ctx, q.Space, q.Resume)
}

type ListRequest struct {
	Space string `json:"space_id"`
	Path  string `json:"path"`
}

// RunListRequest live-lists one directory through the enabled owner service.
// The listing remembers observed files into the index, exactly like a scan;
// results are capped at 500 entries with an honest partial flag.
func (s *Service) RunListRequest(ctx context.Context, q ListRequest) (any, error) {
	if q.Space == "" || len(q.Path) > 1024 {
		return nil, errors.New("invalid_list")
	}
	items, e := s.listEntries(ctx, q.Space, q.Path, true)
	if e != nil {
		return nil, e
	}
	partial := len(items) > 500
	if partial {
		items = items[:500]
	}
	files, directories := []File{}, []File{}
	for _, item := range items {
		if item.Directory {
			directories = append(directories, item)
		} else {
			files = append(files, item)
		}
	}
	return map[string]any{"items": files, "directories": directories, "partial": partial, "coverage": "live_listing"}, nil
}

func listError(e error) string {
	switch e.Error() {
	case "disabled":
		return "listing_disabled"
	case "path_not_allowed", "space_not_allowed", "invalid_list":
		return e.Error()
	}
	return "list_failed"
}
func (s *Service) ListenInsights(socket string) (*http.Server, error) {
	if _, e := os.Lstat(socket); e == nil {
		return nil, errors.New("insights_socket_exists")
	}
	listener, e := net.Listen("unix", socket)
	if e != nil {
		return nil, errors.New("insights_socket_unavailable")
	}
	if e = os.Chmod(socket, 0600); e != nil {
		listener.Close()
		return nil, errors.New("insights_socket_not_private")
	}
	mux := http.NewServeMux()
	s.localFileRoutes(mux)
	s.remoteShareRoutes(mux)
	s.localCreativeRoutes(mux)
	s.localPreviewRoutes(mux)
	s.startRemoteShares()
	mux.HandleFunc("POST /export", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Kind string `json:"kind"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&q) != nil || q.Kind != "audit_export" && q.Kind != "export" {
			http.Error(w, "invalid_export", 400)
			return
		}
		if e := s.RecordInsightExport(q.Kind); e != nil {
			http.Error(w, "audit_unavailable", 503)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /scan", func(w http.ResponseWriter, r *http.Request) {
		var q ScanRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
			http.Error(w, "invalid_scan", 400)
			return
		}
		out, e := s.RunScanRequest(r.Context(), q)
		w.Header().Set("Content-Type", "application/json")
		if e != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "scan_request_failed"}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": out})
	})
	mux.HandleFunc("POST /list", func(w http.ResponseWriter, r *http.Request) {
		var q ListRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
			http.Error(w, "invalid_list", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		out, e := s.RunListRequest(ctx, q)
		w.Header().Set("Content-Type", "application/json")
		if e != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": listError(e)}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": out})
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(listener)
	return srv, nil
}

func (s *Service) RecordInsightExport(kind string) error {
	return s.auditLocal("", "insights."+kind, "", "success")
}
