package bridge

import (
	"context"
	"testing"
	"time"
)

func TestProvenanceSurvivesObservationAndRetry(t *testing.T) {
	s, _, local, _ := setup(t)
	source := fixture(t, local)
	meta := Provenance{Caller: "report-agent", TaskRef: "task:report-1"}
	op, err := s.Upload(context.Background(), "creative", "outputs", source, "report.png", "origin", meta)
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" {
		t.Fatal(op.Error)
	}
	if _, err = s.List(context.Background(), "creative", "outputs"); err != nil {
		t.Fatal(err)
	}
	file, err := s.Stat(context.Background(), op.FileRef)
	if err != nil || file.Provenance != meta {
		t.Fatalf("lost provenance: %v", err)
	}
	replay, err := s.Upload(context.Background(), "creative", "outputs", source, "report.png", "origin", meta)
	if err != nil || replay.ID != op.ID {
		t.Fatal("replay lost original operation")
	}
	if _, err = s.Upload(context.Background(), "creative", "elsewhere", source, "report.png", "origin", meta); err == nil {
		t.Fatal("changed target accepted")
	}
	if _, err = s.Upload(context.Background(), "creative", "outputs", source, "report.png", "origin", Provenance{Caller: "another"}); err == nil {
		t.Fatal("changed provenance accepted")
	}
	if _, err = s.Upload(context.Background(), "creative", "outputs", source, "report.png", "invalid", Provenance{TaskRef: "https://secret.example/?token=private"}); err == nil {
		t.Fatal("URL provenance accepted")
	}
	files, err := s.Search("creative", "report", "outputs", SearchOptions{MIME: "image/png", After: time.Now().Add(-time.Hour)})
	if err != nil || len(files) != 1 {
		t.Fatal("filtered discovery failed")
	}
	files, err = s.Search("creative", "report", "outputs", SearchOptions{After: time.Now().Add(time.Hour)})
	if err != nil || len(files) != 0 {
		t.Fatal("time filter failed")
	}
}
