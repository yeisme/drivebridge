package service_test

import (
	"fmt"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestBrowserBoundedPageSizePreservesLegacyDefault(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	rows := make([]service.Directory, 55)
	for i := range rows {
		rows[i] = service.Directory{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: fmt.Sprintf("page-%03d", i)}
	}
	if err := f.Service.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var page struct {
		Entries []service.BrowserEntry `json:"entries"`
		Next    string                 `json:"next_cursor"`
		Total   int                    `json:"total"`
	}
	base := "/browser?space_id=" + h.space.ID
	if status := h.call(t, "GET", base, nil, &page); status != 200 || len(page.Entries) != 55 || page.Next != "" {
		t.Fatalf("legacy page: status=%d rows=%d cursor=%q", status, len(page.Entries), page.Next)
	}
	if status := h.call(t, "GET", base+"&limit=50", nil, &page); status != 200 || len(page.Entries) != 50 || page.Total != 55 || page.Next != "50" {
		t.Fatalf("first page: status=%d rows=%d cursor=%q", status, len(page.Entries), page.Next)
	}
	last := page.Entries[49].Path
	if status := h.call(t, "GET", base+"&limit=50&cursor="+page.Next, nil, &page); status != 200 || len(page.Entries) != 5 || page.Next != "" || page.Entries[0].Path <= last {
		t.Fatalf("second page: status=%d rows=%d cursor=%q", status, len(page.Entries), page.Next)
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=201", "limit=", "limit=invalid", "limit=50&limit=100"} {
		if status := h.call(t, "GET", base+"&"+query, nil, nil); status != 400 {
			t.Errorf("invalid page size %q: status=%d", query, status)
		}
	}
}
