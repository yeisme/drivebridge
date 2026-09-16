package bridge

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/yeisme/drivebridge-server/sdk"
)

// The local viewer consumes the remote creative facade using the configured
// user connection. It never executes image/video providers itself.
func (s *Service) localCreativeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /files/creative", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fail := func(err error) {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": localFileError(err)}})
		}
		if !s.Enabled {
			fail(errors.New("disabled"))
			return
		}
		var q struct {
			Path   string          `json:"path"`
			Method string          `json:"method"`
			Key    string          `json:"key"`
			Body   json.RawMessage `json:"body"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&q) != nil {
			fail(errors.New("invalid_creative_request"))
			return
		}
		allowed := q.Path == "/creative/connections" && (q.Method == "GET" || q.Method == "POST") || q.Path == "/creative/submit" && q.Method == "POST" || q.Path == "/creative/receipts" && q.Method == "GET"
		if strings.HasPrefix(q.Path, "/creative/receipts/") {
			rest := strings.TrimPrefix(q.Path, "/creative/receipts/")
			id := strings.TrimSuffix(rest, "/save")
			safe := len(id) == 48
			for _, r := range id {
				safe = safe && (r >= 'a' && r <= 'f' || r >= '0' && r <= '9')
			}
			allowed = safe && (rest == id && q.Method == "GET" || rest == id+"/save" && q.Method == "POST")
		}
		if !allowed {
			fail(errors.New("invalid_creative_request"))
			return
		}
		client, err := s.shareClient()
		if err != nil {
			fail(err)
			return
		}
		var body any
		if len(q.Body) > 0 && string(q.Body) != "null" {
			if json.Unmarshal(q.Body, &body) != nil {
				fail(errors.New("invalid_creative_request"))
				return
			}
		}
		if q.Path == "/creative/submit" {
			var submission map[string]json.RawMessage
			if json.Unmarshal(q.Body, &submission) != nil {
				fail(errors.New("invalid_creative_request"))
				return
			}
			var inputs []struct {
				Ref     string `json:"file_id"`
				Version string `json:"version_id"`
				Role    string `json:"role"`
			}
			var target string
			var rights, cost bool
			json.Unmarshal(submission["space_id"], &target)
			json.Unmarshal(submission["confirm_rights"], &rights)
			json.Unmarshal(submission["confirm_cost"], &cost)
			if json.Unmarshal(submission["inputs"], &inputs) != nil || len(inputs) > 8 || target == "" || !rights || !cost || q.Key == "" {
				fail(errors.New("creative_confirmation_required"))
				return
			}
			remoteInputs := []map[string]string{}
			for _, input := range inputs {
				f, err := s.Stat(r.Context(), input.Ref)
				if err != nil || f.Trashed != nil || f.Version != input.Version {
					fail(errors.New("file_version_changed"))
					return
				}
				space, err := s.Space(f.SpaceID)
				if err != nil {
					fail(err)
					return
				}
				source := &localSeeker{size: f.Size, open: func() (io.ReadCloser, error) {
					if _, err := s.Stat(r.Context(), f.Ref); err != nil {
						return nil, err
					}
					return s.providerFor(space).Open(r.Context(), space, f)
				}}
				op, err := client.Upload(r.Context(), sdk.UploadRequest{SpaceID: target, Path: path.Join("Materials", strings.TrimPrefix(f.Ref, "drivebridge://file/")[:12]+"-"+f.Name), Caller: "drivebridge-client"}, "material:"+target+":"+f.Ref, source, nil)
				source.close()
				if err != nil {
					fail(err)
					return
				}
				remoteInputs = append(remoteInputs, map[string]string{"file_id": op.FileID, "version_id": op.VersionID, "role": input.Role})
			}
			submission["inputs"], _ = json.Marshal(remoteInputs)
			body = submission
		}
		var result any
		if err = client.JSON(r.Context(), q.Method, "/api/v1"+q.Path, q.Key, body, &result); err != nil {
			fail(err)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": result})
	})
}
