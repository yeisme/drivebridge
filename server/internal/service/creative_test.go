package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestCreativeEikonaHandoffAndSaveback(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	picture := image.NewRGBA(image.Rect(0, 0, 8, 8))
	picture.Set(2, 2, color.RGBA{255, 0, 0, 255})
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, picture); err != nil {
		t.Fatal(err)
	}
	input, err := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "reference.png"}, "creative-image", bytes.NewReader(pngBytes.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	var submissions atomic.Int64
	var runID string
	var owner *httptest.Server
	owner = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data any) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"data": data})
		}
		if r.Header.Get("Authorization") != "Bearer fixture-consumer-key" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/asset-uploads":
			respond(map[string]any{"session_id": "upl_0123456789abcdef", "method": "PUT", "url": owner.URL + "/api/v1/asset-uploads/upl_0123456789abcdef", "authorization": "bearer"})
		case r.Method == "PUT":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, pngBytes.Bytes()) {
				t.Error("source bytes changed")
			}
			w.WriteHeader(204)
		case strings.HasSuffix(r.URL.Path, "/complete"):
			respond(map[string]string{"uri": "eikona://asset/fixture"})
		case r.URL.Path == "/api/v1/generation":
			var q struct {
				RunID string   `json:"run_id"`
				Refs  []string `json:"reference_images"`
				Model string   `json:"model_ref"`
			}
			json.NewDecoder(r.Body).Decode(&q)
			if len(q.Refs) != 1 || q.Refs[0] != "eikona://asset/fixture" || q.Model != "openai/gpt-5.4-image-2" {
				t.Error("incorrect reference or model")
			}
			runID = q.RunID
			submissions.Add(1)
			respond(map[string]string{"run_id": runID})
		case strings.HasPrefix(r.URL.Path, "/api/v1/runs/"):
			respond(map[string]any{"run_id": runID, "status": "succeeded", "resource_uris": []string{"eikona://artifact/fixture-result"}})
		case strings.HasSuffix(r.URL.Path, "/download-grants"):
			respond(map[string]any{"download_url": "/api/v1/downloads/result", "expires_at": time.Now().Add(time.Minute)})
		case r.URL.Path == "/api/v1/downloads/result":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes.Bytes())
		default:
			w.WriteHeader(404)
		}
	}))
	defer owner.Close()
	var connection service.CreativeConnection
	if got := h.call(t, "POST", "/creative/connections", map[string]string{"kind": "eikona", "name": "Fixture", "project_ref": "project-a", "url": owner.URL, "token": "fixture-consumer-key"}, &connection); got != 200 {
		t.Fatal("connect", got)
	}
	request := service.CreativeSubmit{ConnectionID: connection.ID, SpaceID: h.space.ID, Path: "生成结果", Prompt: "fixture task", Inputs: []service.CreativeInput{{FileID: input.FileID, VersionID: input.VersionID, Role: "reference"}}, ConfirmRights: true, ConfirmCost: true}
	submit := func(q service.CreativeSubmit, key string) (int, service.CreativeReceipt) {
		b, _ := json.Marshal(q)
		req, _ := http.NewRequest("POST", h.base+"/api/v1/creative/submit", bytes.NewReader(b))
		req.Header.Set("Origin", h.base)
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Idempotency-Key", key)
		resp, e := h.http.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		var out struct {
			Data service.CreativeReceipt `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out.Data
	}
	unconfirmed := request
	unconfirmed.ConfirmCost = false
	if status, _ := submit(unconfirmed, "original"); status == 200 || submissions.Load() != 0 {
		t.Fatal("unconfirmed submission")
	}
	status, receipt := submit(request, "original")
	if status != 200 || receipt.Status != "submitted" {
		t.Fatalf("submit %d %+v", status, receipt)
	}
	if status, again := submit(request, "original"); status != 200 || again.ID != receipt.ID || submissions.Load() != 1 {
		t.Fatal("duplicate generation")
	}
	changed := request
	changed.Prompt = "changed"
	if status, _ := submit(changed, "original"); status != 409 {
		t.Fatal("conflict accepted")
	}
	var result struct {
		State     string   `json:"state"`
		Artifacts []string `json:"artifacts"`
	}
	if status := h.call(t, "GET", "/creative/receipts/"+receipt.ID, nil, &result); status != 200 || len(result.Artifacts) != 1 {
		t.Fatal("result", status)
	}
	var saved service.Operation
	if status := h.call(t, "POST", "/creative/receipts/"+receipt.ID+"/save", map[string]string{"artifact_ref": result.Artifacts[0]}, &saved); status != 200 || saved.Status != "completed" {
		t.Fatal("save", status, saved.Status)
	}
	var savedFile struct {
		File service.File `json:"file"`
	}
	h.call(t, "GET", "/files/"+saved.FileID, nil, &savedFile)
	if !savedFile.File.ReviewPending {
		t.Fatal("result missing review quarantine")
	}
	if status := h.call(t, "POST", "/creative/receipts/"+receipt.ID+"/save", map[string]string{"artifact_ref": "eikona://artifact/unrelated"}, nil); status != 403 {
		t.Fatal("foreign result admitted")
	}
	var again service.Operation
	h.call(t, "POST", "/creative/receipts/"+receipt.ID+"/save", map[string]string{"artifact_ref": result.Artifacts[0]}, &again)
	if again.ID != saved.ID || submissions.Load() != 1 {
		t.Fatal("saveback repeated generation or upload")
	}
}
