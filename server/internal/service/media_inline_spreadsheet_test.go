package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestInlineSpreadsheetMedia(t *testing.T) {
	fixture := rig(t)
	alice, _ := login(t, fixture, "alice")
	bob, _ := login(t, fixture, "bob")
	_, client := agent(t, alice, "", "read,write")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("xl/workbook.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(entry, `<workbook/>`); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, forcedMIME, expectedMIME string
		data                           []byte
		allowed                        bool
	}{
		{"book.xlsx", "", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", archive.Bytes(), true},
		{"generic.XLSX", "application/octet-stream", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", archive.Bytes(), true},
		{"typed", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", archive.Bytes(), true},
		{"table.csv", "application/octet-stream", "text/csv", []byte("编号,值\n001,2"), true},
		{"table.tsv", "application/octet-stream", "text/tab-separated-values", []byte("编号\t值\n001\t2"), true},
		{"csv-mime", "application/csv", "application/csv", []byte("a,b\n1,2"), true},
		{"archive.zip", "", "", archive.Bytes(), false},
		{"other.bin", "application/octet-stream", "", []byte{0, 1, 2, 3}, false},
		{"document.docx", "", "", archive.Bytes(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			uploaded, err := client.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: test.name}, "spreadsheet-"+test.name, bytes.NewReader(test.data), nil)
			if err != nil {
				t.Fatal(err)
			}
			if test.forcedMIME != "" {
				if err := fixture.Service.DB.Model(&service.Version{}).Where("id = ?", uploaded.VersionID).Update("mime", test.forcedMIME).Error; err != nil {
					t.Fatal(err)
				}
			}
			endpoint := alice.base + "/api/v1/files/" + uploaded.FileID + "/media?version=" + uploaded.VersionID
			response, err := alice.http.Get(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if !test.allowed {
				if response.StatusCode == http.StatusOK {
					t.Fatal("unrelated binary admitted")
				}
				return
			}
			if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != test.expectedMIME {
				t.Fatalf("response %d MIME %s", response.StatusCode, response.Header.Get("Content-Type"))
			}
			if response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(response.Header.Get("Content-Security-Policy"), "sandbox") {
				t.Fatal("missing preview isolation headers")
			}
			body, err := io.ReadAll(response.Body)
			if err != nil || !bytes.Equal(body, test.data) {
				t.Fatal("original bytes changed", err)
			}
			foreign, err := bob.http.Get(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			foreign.Body.Close()
			if foreign.StatusCode != http.StatusNotFound {
				t.Fatalf("foreign preview status %d", foreign.StatusCode)
			}
			interactive, err := alice.http.Get(endpoint + "&preview=interactive")
			if err != nil {
				t.Fatal(err)
			}
			interactive.Body.Close()
			if interactive.StatusCode == http.StatusOK {
				t.Fatal("spreadsheet acquired interactive document access")
			}
		})
	}
}
