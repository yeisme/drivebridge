package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFPShareReceiverFirstStorageReadAdmission(t *testing.T) {
	for _, scenario := range []string{"truncated-range", "empty-range", "valid-range", "head", "invalid-range", "valid-empty", "invalid-empty"} {
		t.Run(scenario, func(t *testing.T) {
			s, share, secret := receiverFixture(t)
			// Exercise the real S3 HTTP adapter. The service receives a reader
			// successfully before an empty/truncated object fails its first Read.
			objects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet { t.Errorf("unexpected storage request %s", r.Method); w.WriteHeader(500); return }
				length := 4
				if scenario == "empty-range" || scenario == "valid-empty" { length = 0 }
				if scenario == "invalid-empty" { length = 1 }
				w.Header().Set("Content-Length", strconv.Itoa(length))
				if r.Header.Get("Range") != "" {
					w.Header().Set("Content-Range", "bytes 0-3/8")
					w.WriteHeader(http.StatusPartialContent)
				}
				if scenario == "valid-range" || scenario == "head" { _, _ = io.WriteString(w, "data") }
				if scenario == "invalid-empty" { _, _ = io.WriteString(w, "x") }
			}))
			defer objects.Close()
			config := S3Config{Endpoint: objects.URL, Region: "us-east-1", Bucket: "receiver-fixture", AccessKey: "fixture-key", SecretKey: "fixture-secret"}
			sealed, err := s.seal(share.Owner, config)
			if err != nil { t.Fatal(err) }
			connection := Connection{ID: ID(), Owner: share.Owner, Kind: "s3", Secret: sealed}
			space := Space{ID: ID(), Owner: share.Owner, ConnectionID: connection.ID}
			file := File{ID: ID(), Owner: share.Owner, SpaceID: space.ID, Path: "range.txt", VersionID: ID(), Size: 8}
			if scenario == "valid-empty" || scenario == "invalid-empty" { file.Size = 0 }
			version := Version{ID: file.VersionID, Owner: share.Owner, FileID: file.ID, ConnectionID: connection.ID, ObjectKey: "object", Size: file.Size, MIME: "text/plain"}
			for _, row := range []any{&connection, &space, &file, &version, &ShareItem{ShareID: share.ID, FileID: file.ID, VersionID: file.VersionID, Path: file.Path}} {
				if err = s.DB.Create(row).Error; err != nil { t.Fatal(err) }
			}
			share.SpaceID = space.ID
			if err = s.DB.Save(&share).Error; err != nil { t.Fatal(err) }
			opened := receiverOpen(t, s, share, secret)
			if scenario == "truncated-range" || scenario == "empty-range" {
				store, err := NewS3(config, true)
				if err != nil { t.Fatal(err) }
				body, err := store.(*s3Store).GetRange(context.Background(), version.ObjectKey, 0, 3)
				if err != nil { t.Fatal("fixture must fail after returning reader", err) }
				var probe [4]byte
				n, readErr := body.Read(probe[:])
				body.Close()
				if n != 0 || readErr == nil { t.Fatal("fixture first-read boundary", n, readErr) }
			}
			method := http.MethodGet
			if scenario == "head" { method = http.MethodHead }
			request := httptest.NewRequest(method, "/api/v1/shared/"+share.ID+"/files/"+file.ID+"/content?download=true", nil)
			request.AddCookie(&http.Cookie{Name: shareCookie(share.ID), Value: opened.Token})
			if file.Size > 0 { request.Header.Set("Range", "bytes=0-3") }
			if scenario == "invalid-range" { request.Header.Set("Range", "bytes=999-") }
			response := httptest.NewRecorder()
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = s.serveShareReceiverContent(response, request, share, file, true)
			}()
			failedRead := scenario == "truncated-range" || scenario == "empty-range"
			if failedRead && recovered != http.ErrAbortHandler { t.Fatal("storage failure did not abort", recovered, err) }
			if !failedRead && recovered != nil { t.Fatal("unexpected stream abort", recovered) }
			if scenario == "invalid-empty" {
				if err == nil || err.Error() != "content_mismatch" { t.Fatal("invalid empty object", err) }
			} else if !failedRead && err != nil { t.Fatal(err) }
			charged := scenario == "valid-range" || scenario == "valid-empty"
			if scenario == "valid-range" {
				if response.Code != http.StatusPartialContent || response.Body.String() != "data" { t.Fatal("range response changed", response.Code, response.Body.String()) }
				if err = s.serveShareReceiverContent(httptest.NewRecorder(), request, share, file, true); err != nil { t.Fatal("range resume", err) }
			}
			if err = s.DB.First(&share, "id = ?", share.ID).Error; err != nil { t.Fatal(err) }
			var tuples int64
			if err = s.DB.Model(&ShareDownload{}).Where("share_id = ?", share.ID).Count(&tuples).Error; err != nil { t.Fatal(err) }
			want := int64(0)
			if charged { want = 1 }
			if share.DownloadsUsed != want || tuples != want { t.Fatalf("quota/tuple after %s: %d/%d want %d", scenario, share.DownloadsUsed, tuples, want) }
			if !charged && response.Body.Len() != 0 { t.Fatal("failed stream sent content") }
		})
	}
}
