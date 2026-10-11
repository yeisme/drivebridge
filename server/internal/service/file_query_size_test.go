package service_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestFileQuerySizePaginationAndCollection(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	// Metadata-only query fixture: no fabricated provider upload evidence.
	rows := make([]service.File, 205)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range rows {
		rows[i] = service.File{ID: fmt.Sprintf("%048x", i+1), Owner: p.UserID, SpaceID: h.space.ID, Path: fmt.Sprintf("sizes/%03d.txt", i), VersionID: fmt.Sprintf("%048x", i+1000), Size: int64((i * 17) % 23), Updated: stamp}
	}
	if err := f.Service.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, direction := range []string{"asc", "desc"} {
		t.Run(direction, func(t *testing.T) {
			expected := slices.Clone(rows)
			slices.SortFunc(expected, func(a, b service.File) int {
				result := 0
				if a.Size < b.Size {
					result = -1
				} else if a.Size > b.Size {
					result = 1
				} else if a.ID < b.ID {
					result = -1
				} else if a.ID > b.ID {
					result = 1
				}
				if direction == "desc" {
					return -result
				}
				return result
			})
			q := service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Sort: []service.FileQuerySort{{Field: "size_bytes", Direction: direction}}, Limit: 7}
			ids := []string{}
			seen := map[string]bool{}
			finished := false
			for range 40 {
				page, err := f.Service.QueryFiles(p, q)
				if err != nil {
					t.Fatal(err)
				}
				for _, item := range page.Items {
					if seen[item.FileID] {
						t.Fatal("duplicate size page item")
					}
					seen[item.FileID] = true
					ids = append(ids, item.FileID)
				}
				if page.NextCursor == "" {
					finished = true
					break
				}
				q.Cursor = page.NextCursor
			}
			if !finished || len(ids) != len(expected) {
				t.Fatalf("size pagination lost rows: %d/%d", len(ids), len(expected))
			}
			for i, id := range ids {
				if id != expected[i].ID {
					t.Fatalf("wrong size/ID ordering at %d", i)
				}
			}
			raw := json.RawMessage(fmt.Sprintf(`{"mode":"exact","sort":[{"field":"size_bytes","direction":%q}]}`, direction))
			collection, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Sizes", Kind: "rule", Scope: "private", Rule: raw}, "size-rule-"+direction)
			if err != nil {
				t.Fatal("size rule unavailable", err)
			}
			view, err := f.Service.ReadFileCollection(p, collection.CollectionID)
			if err != nil || len(view.Items) != len(expected) || view.Coverage != "complete" {
				t.Fatal("multi-page size rule incomplete", err)
			}
			for i, item := range view.Items {
				if item.FileID != expected[i].ID {
					t.Fatal("collection size order differs")
				}
			}
		})
	}
	q := service.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Sort: []service.FileQuerySort{{Field: "size_bytes", Direction: "asc"}}, Limit: 1}
	first, err := f.Service.QueryFiles(p, q)
	if err != nil || first.NextCursor == "" {
		t.Fatal("missing cursor", err)
	}
	q.Cursor = first.NextCursor
	changed := q
	changed.Sort = []service.FileQuerySort{{Field: "size_bytes", Direction: "desc"}}
	if _, err = f.Service.QueryFiles(p, changed); err == nil || err.Error() != "query_changed" {
		t.Fatal("changed sort reused cursor", err)
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", rows[0].ID).Updates(map[string]any{"size": 99, "updated": stamp.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.QueryFiles(p, q); err == nil || err.Error() != "query_changed" {
		t.Fatal("stale size cursor accepted", err)
	}
}
