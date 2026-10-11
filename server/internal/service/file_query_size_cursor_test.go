package service

import "testing"

func TestFileQuerySizeRejectsLegacyCursor(t *testing.T) {
	s, _, p := blobRig(t)
	var space Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}
	uploaded := uploadBytes(t, s, p, space.ID, "cursor.txt", []byte("fixture"))
	revision, err := s.catalogueRevision(p.UserID)
	if err != nil {
		t.Fatal(err)
	}
	q := FileQueryRequest{Mode: "exact", SpaceIDs: []string{space.ID}, Sort: []FileQuerySort{{Field: "size_bytes", Direction: "asc"}}, Limit: 1}
	for _, value := range []string{"2026-01-01T00:00:00Z", "-1", "01", "9223372036854775808", "1.5", ""} {
		cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: canonicalQueryJSON(q), CatalogueRevision: revision, LastKey: value + "|" + uploaded.FileID, Kind: "files_query"})
		if err != nil {
			t.Fatal(err)
		}
		q.Cursor = cursor
		if _, err = s.QueryFiles(p, q); err == nil || err.Error() != "query_changed" {
			t.Fatalf("invalid size key not rejected: %v", err)
		}
	}
}
