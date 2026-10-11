package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestCreativeScaenaUsesUploadSessionAndExplicitBudget(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	var source bytes.Buffer
	png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	sum := sha256.Sum256(source.Bytes())
	digest := hex.EncodeToString(sum[:])
	input, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "video-reference.png"}, "video-input", bytes.NewReader(source.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	var submissions atomic.Int64
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data any) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"data": data})
		}
		if r.Header.Get("Authorization") != "Bearer fixture-video-consumer" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/api/v1/projects/video-project/upload-sessions":
			respond(map[string]any{"session_ref": "upload-session:fixture", "part_size": source.Len(), "total_size": source.Len()})
		case r.Method == "PUT":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, source.Bytes()) || r.Header.Get("Content-Digest") != "sha256="+digest {
				t.Error("incorrect media part or digest")
			}
			respond(map[string]bool{"received": true})
		case strings.HasSuffix(r.URL.Path, ":complete"):
			respond(map[string]string{"upload_ref": "upload:fixture"})
		case strings.HasSuffix(r.URL.Path, "/video-tasks"):
			var q map[string]any
			json.NewDecoder(r.Body).Decode(&q)
			if q["model"] != nil || q["binding_id"] != "fixture-binding" || q["reference_upload_ref"] != "upload:fixture" || q["max_cost_micro_usd"] != float64(1000000) || q["confirmed"] != true || q["subject_mode"] != "synthetic_virtual_person" {
				t.Error("incorrect video admission request")
			}
			submissions.Add(1)
			respond(map[string]string{"submission_ref": "video-task:fixture"})
		case strings.Contains(r.URL.Path, "/video-tasks/"):
			respond(map[string]string{"status": "running", "submission_ref": "video-task:fixture"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer owner.Close()
	var conn service.CreativeConnection
	if status := h.call(t, "POST", "/creative/connections", map[string]string{"kind": "scaena", "name": "Video fixture", "url": owner.URL, "project_ref": "video-project", "token": "fixture-video-consumer"}, &conn); status != 200 {
		t.Fatal(status)
	}
	q := service.CreativeSubmit{ConnectionID: conn.ID, SpaceID: h.space.ID, Path: "video-results", Model: "fixture-binding", Prompt: "fixture motion", Inputs: []service.CreativeInput{{FileID: input.FileID, VersionID: input.VersionID}}, ConfirmRights: true, ConfirmCost: true, Duration: 5, SubjectMode: "synthetic_virtual_person", MaxCostMicroUSD: 1000000}
	submit := func(q service.CreativeSubmit) (int, service.CreativeReceipt) {
		body, _ := json.Marshal(q)
		req, _ := http.NewRequest("POST", h.base+"/api/v1/creative/submit", bytes.NewReader(body))
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", "video-once")
		response, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result struct {
			Data service.CreativeReceipt `json:"data"`
		}
		json.NewDecoder(response.Body).Decode(&result)
		return response.StatusCode, result.Data
	}
	withoutBudget := q
	withoutBudget.MaxCostMicroUSD = 0
	if status, _ := submit(withoutBudget); status == 200 || submissions.Load() != 0 {
		t.Fatal("missing video budget admitted")
	}
	status, receipt := submit(q)
	if status != 200 || receipt.TaskRef != "video-task:fixture" || receipt.Status != "submitted" {
		t.Fatal("video submission", status, receipt.Status)
	}
	if status, again := submit(q); status != 200 || again.ID != receipt.ID || submissions.Load() != 1 {
		t.Fatal("video duplicate")
	}
}
