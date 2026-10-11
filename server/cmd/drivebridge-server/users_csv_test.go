package main

import (
	"os"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestParseUserCSVModes(t *testing.T) {
	// Plain single-column list: BOM, blank lines and padding tolerated.
	plain, e := parseUserCSV("\ufeff\n  张三  \n\n赵六\n")
	if e != nil || len(plain) != 2 || plain[0].Name != "张三" || plain[1].Name != "赵六" {
		t.Fatalf("plain list: %v %+v", e, plain)
	}
	// Header table with per-user options; unnamed extra columns are notes.
	header, e := parseUserCSV("name,days,admin,备注\n张三,90,,同事\n李四,,true,\n")
	if e != nil || len(header) != 2 || header[0].Days != 90 || header[0].Admin || !header[1].Admin {
		t.Fatalf("header table: %v %+v", e, header)
	}
	// Positional table without header.
	pos, e := parseUserCSV("张三,30,1\n")
	if e != nil || len(pos) != 1 || pos[0].Days != 30 || !pos[0].Admin {
		t.Fatalf("positional: %v %+v", e, pos)
	}
	// Operator-supplied keys round-trip through the key column.
	supplied, e := parseUserCSV("name,key\n甲,dbu_" + repeat("ab", 48) + "\n")
	if e != nil || len(supplied) != 1 || supplied[0].Key != "dbu_"+repeat("ab", 48) {
		t.Fatalf("supplied key: %v %+v", e, supplied)
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestParseUserCSVRejectsBadRows(t *testing.T) {
	bad := []string{
		"",
		"\n \n",
		"name,days\n甲,x\n",      // non-numeric days
		"name,days\n甲,-1\n",     // negative days
		"name,admin\n甲,maybe\n", // unknown admin word
	}
	for _, raw := range bad {
		if out, e := parseUserCSV(raw); e == nil {
			t.Fatalf("expected rejection for %q, got %+v", raw, out)
		}
	}
}

func TestWriteIssuedCSVPermissionsAndShape(t *testing.T) {
	path := t.TempDir() + "/dist.csv"
	issued := []service.UserKeyIssued{{UserID: "u1", Name: "张三", Key: "dbu_x", Admin: false}}
	if e := writeIssuedCSV(path, issued); e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("distribution file must be 0600, got %v", info.Mode().Perm())
	}
}
