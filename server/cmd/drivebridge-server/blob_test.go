package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestBlobStatAndGCDryRunCLI(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// Seed one referenced and one unreferenced blob record.
	s, err := service.Open(service.Config{State: dir, Enabled: true, Development: true})
	if err != nil {
		t.Fatal(err)
	}
	sha := "abcd"
	for i := 0; i < 31; i++ {
		sha += "abcd"
	}
	sha = sha[:64]
	seed := []service.Blob{
		{ID: "b1", SHA256: sha, Owner: "owner", ConnectionID: "platform", ObjectKey: "instance/blobs/" + sha, Size: 10, Refs: 2},
		{ID: "b2", SHA256: sha, Owner: "owner", SpaceID: "space", ConnectionID: "platform", ObjectKey: "instance/blobs/space/" + sha, Size: 5, Refs: 0, Encrypted: true},
	}
	for _, b := range seed {
		if err := s.DB.Create(&b).Error; err != nil {
			t.Fatal(err)
		}
	}
	s.Close()

	out, err := capture(t, func() error { return runCLI("blob", "stat", sha, "--state", dir, "--json") })
	if err != nil {
		t.Fatalf("blob stat: %v\n%s", err, out)
	}
	var statEnv struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &statEnv); err != nil || statEnv.Data.Count != 2 {
		t.Fatalf("blob stat output: %v %q", err, out)
	}
	if out, err = capture(t, func() error { return runCLI("blob", "stat", "ZZZ", "--state", dir, "--json") }); err == nil {
		t.Fatalf("invalid digest must fail: %s", out)
	}

	out, err = capture(t, func() error { return runCLI("blob", "gc", "--state", dir, "--json") })
	if err != nil {
		t.Fatalf("blob gc: %v\n%s", err, out)
	}
	var gcEnv struct {
		Data struct {
			DryRun      bool  `json:"dry_run"`
			Reclaimable int   `json:"reclaimable_blobs"`
			Bytes       int64 `json:"reclaimable_bytes"`
			Removed     int   `json:"removed"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &gcEnv); err != nil || !gcEnv.Data.DryRun || gcEnv.Data.Reclaimable != 1 || gcEnv.Data.Bytes != 5 || gcEnv.Data.Removed != 0 {
		t.Fatalf("blob gc dry-run output: %v %q", err, out)
	}
	// Dry run left every row intact.
	s, err = service.Open(service.Config{State: dir, Enabled: true, Development: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var count int64
	if err := s.DB.Model(&service.Blob{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("dry run must not delete rows: %v %d", err, count)
	}
	report, err := s.BlobGC(context.Background(), true)
	if err != nil || report["reclaimable_blobs"] != 1 {
		t.Fatalf("service dry run: %v %+v", err, report)
	}
}
