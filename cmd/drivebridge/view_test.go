package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"github.com/yeisme/drivebridge/internal/bridge"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestViewerSessionOriginAndReadOnlyBoundary(t *testing.T) {
	dir := t.TempDir()
	db, e := gorm.Open(sqlite.Open(filepath.Join(dir, "view.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	store, e := insights.Open(db)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { store.Close(); raw, _ := db.DB(); raw.Close() }()
	os.WriteFile(filepath.Join(dir, "insights.html"), []byte("<!doctype html><title>fixture view</title>"), 0600)
	origin := "http://127.0.0.1:34567"
	handler := viewerHandler(&cli{}, store, dir, origin, "private-bootstrap", "private-session", "private-csrf")
	request := func(method, path, body, host, originHeader, cookie, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, origin+path, bytes.NewBufferString(body))
		if host != "" {
			r.Host = host
		}
		if originHeader != "" {
			r.Header.Set("Origin", originHeader)
		}
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/v1/usage/summary", "", "", "", "", ""); w.Code != 401 {
		t.Fatal("unauthenticated insights exposed")
	}
	if w := request("GET", "/insights.html", "", "attacker.example", "", "", ""); w.Code != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
	if w := request("POST", "/api/v1/local-session", `{"launch":"private-bootstrap"}`, "", "http://attacker.example", "", ""); w.Code != 403 {
		t.Fatal("cross-origin bootstrap accepted")
	}
	w := request("POST", "/api/v1/local-session", `{"launch":"private-bootstrap"}`, "", origin, "", "")
	if w.Code != 200 || len(w.Result().Cookies()) != 1 || !w.Result().Cookies()[0].HttpOnly {
		t.Fatal("session bootstrap failed")
	}
	cookie := w.Result().Cookies()[0].String()
	if w = request("POST", "/api/v1/local-session", `{"launch":"private-bootstrap"}`, "", origin, "", ""); w.Code != 401 {
		t.Fatal("bootstrap replay accepted")
	}
	w = request("GET", "/api/v1/usage/summary", "", "", "", cookie, "")
	if w.Code != 200 {
		t.Fatalf("authenticated stats failed %d", w.Code)
	}
	var out map[string]any
	if json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal("invalid JSON")
	}
	if w = request("POST", "/api/v1/files/anything/trash", `{}`, "", origin, cookie, "private-csrf"); w.Code != 403 {
		t.Fatal("file write route exposed")
	}
	if w = request("POST", "/api/v1/usage/scans", `{}`, "", origin, cookie, ""); w.Code != 403 {
		t.Fatal("scan skipped CSRF")
	}
}

func TestViewerSpacesBackendProjection(t *testing.T) {
	dir := t.TempDir()
	db, e := gorm.Open(sqlite.Open(filepath.Join(dir, "view.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	store, e := insights.Open(db)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { store.Close(); raw, _ := db.DB(); raw.Close() }()
	if e := db.AutoMigrate(&bridge.Space{}); e != nil {
		t.Fatal(e)
	}
	for _, space := range []bridge.Space{
		{ID: "creative", Remote: "mydrive", DriveID: "drive", RootID: "root", LocalRoot: dir, MaxBytes: 1 << 30},
		{ID: "nas", Remote: "nas", Backend: "s3", DriveID: "", RootID: "Projects", LocalRoot: dir, MaxBytes: 1 << 30},
	} {
		if e := db.Create(&space).Error; e != nil {
			t.Fatal(e)
		}
	}
	os.WriteFile(filepath.Join(dir, "insights.html"), []byte("<!doctype html><title>fixture view</title>"), 0600)
	origin := "http://127.0.0.1:34568"
	handler := viewerHandler(&cli{}, store, dir, origin, "private-bootstrap", "private-session", "private-csrf")
	r := httptest.NewRequest("POST", origin+"/api/v1/local-session", bytes.NewBufferString(`{"launch":"private-bootstrap"}`))
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("session bootstrap failed %d", w.Code)
	}
	r = httptest.NewRequest("GET", origin+"/api/v1/spaces", nil)
	r.Header.Set("Cookie", w.Result().Cookies()[0].String())
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("spaces failed %d", w.Code)
	}
	body := w.Body.String()
	for _, expected := range []string{
		`"id":"creative"`, `"backend":"onedrive"`, `"connection_id":"mydrive"`,
		`"id":"nas"`, `"backend":"s3"`, `"connection_id":"nas"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("projection missing %s in %s", expected, body)
		}
	}
}
