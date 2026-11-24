package desktop

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopReaderQueryRejectsAmbiguousAndUnboundedInput(t *testing.T) {
	for _, raw := range []string{"limit=0", "limit=201", "limit=no", "limit=1&limit=2", "token=secret", "cursor=%00", "cursor=%zz", "cursor=" + strings.Repeat("a", 4097)} {
		r := httptest.NewRequest("GET", "/?"+raw, nil)
		if _, err := readerPageQuery(r, "limit", "cursor"); err == nil {
			t.Fatalf("invalid query accepted: %q", raw[:min(len(raw), 40)])
		}
	}
	r := httptest.NewRequest("GET", "/?limit=50&cursor=original&sort=updated_desc", nil)
	q, err := readerPageQuery(r, "limit", "cursor", "sort")
	if err != nil || q.Limit != 50 || q.Cursor != "original" || q.Sort != "updated_desc" {
		t.Fatalf("valid query: %+v %v", q, err)
	}
}

func TestDesktopReaderRoutesAreFixedAndRequirePinnedRevision(t *testing.T) {
	for _, path := range []string{"/v1/desktop/sources/s/notes/n/document", "/v1/desktop/sources/s/notes/n/links"} {
		s, n, _, ok := readerDocumentRoute(path)
		if !ok || s != "s" || n != "n" {
			t.Fatal("valid route rejected")
		}
	}
	for _, path := range []string{"/v1/desktop/sources/s/notes/../document", "/v1/desktop/sources/s/notes/n/document/extra", "/v1/desktop/sources/s/notes/n/delete"} {
		if _, _, _, ok := readerDocumentRoute(path); ok {
			t.Fatal("invalid route accepted")
		}
	}
	f := &facade{}
	for _, query := range []string{"", "revision=latest", "revision=sha256:" + strings.Repeat("z", 64), "revision=sha256:" + strings.Repeat("a", 64) + "&revision=other"} {
		response := httptest.NewRecorder()
		f.readDocument(t.Context(), response, httptest.NewRequest("GET", "/?"+query, nil), "s", "n")
		if response.Code != 400 {
			t.Fatalf("invalid pinned revision reached owner: %d", response.Code)
		}
	}
}
