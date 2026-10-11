package contract

import (
	"net/url"
	"testing"
)

func TestPageQueryBoundsAndDuplicateRejection(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		limit int
		valid bool
	}{
		{"", 50, true}, {"limit=1", 1, true}, {"limit=200", 200, true}, {"limit=0", 0, false}, {"limit=201", 0, false},
		{"limit=", 0, false}, {"limit=-1", 0, false}, {"limit=%2B1", 0, false}, {"limit=1.0", 0, false},
		{"limit=20&limit=30", 0, false}, {"cursor=a&cursor=b", 0, false}, {"limit=999999999999999999999", 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			values, err := url.ParseQuery(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParsePageQuery(values)
			if (err == nil) != tc.valid {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if tc.valid && got.Limit != tc.limit {
				t.Fatalf("limit=%d", got.Limit)
			}
		})
	}
}

func TestPageQueryPreservesOpaqueCursor(t *testing.T) {
	got, err := ParsePageQuery(url.Values{"cursor": {"opaque+/="}})
	if err != nil || got.Cursor != "opaque+/=" {
		t.Fatalf("cursor changed: %+v %v", got, err)
	}
}
