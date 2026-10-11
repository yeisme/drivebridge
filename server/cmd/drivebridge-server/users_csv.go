package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yeisme/drivebridge-server/internal/service"
)

// parseUserCSV turns the operator's table into import entries. A first row
// whose cells match the header vocabulary is treated as a header; otherwise
// columns are positional name[,days[,admin[,key]]]. A single column is just a
// plain name list. BOM, blank lines and surrounding spaces are tolerated.
func parseUserCSV(raw string) ([]service.UserImport, error) {
	raw = strings.TrimPrefix(raw, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("invalid user batch")
	}
	header := map[string]int{}
	first, e := csv.NewReader(strings.NewReader(kept[0])).Read()
	if e != nil {
		return nil, fmt.Errorf("invalid user batch")
	}
	if strings.EqualFold(strings.TrimSpace(first[0]), "name") {
		for i, cell := range first {
			header[strings.ToLower(strings.TrimSpace(cell))] = i
		}
		kept = kept[1:]
	}
	pick := func(row []string, col string, pos int) string {
		if i, ok := header[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		if !headerParsed(header) && pos < len(row) {
			return strings.TrimSpace(row[pos])
		}
		return ""
	}
	out := make([]service.UserImport, 0, len(kept))
	for _, line := range kept {
		row, e := csv.NewReader(strings.NewReader(line)).Read()
		if e != nil || len(row) == 0 {
			return nil, fmt.Errorf("invalid user batch")
		}
		entry := service.UserImport{Name: pick(row, "name", 0), Key: pick(row, "key", 3)}
		if d := pick(row, "days", 1); d != "" {
			var days int
			if _, e := fmt.Sscanf(d, "%d", &days); e != nil || days < 0 {
				return nil, fmt.Errorf("invalid_user_batch")
			}
			entry.Days = days
		}
		switch strings.ToLower(pick(row, "admin", 2)) {
		case "true", "1", "yes":
			entry.Admin = true
		case "", "false", "0", "no":
		default:
			return nil, fmt.Errorf("invalid_user_batch")
		}
		out = append(out, entry)
	}
	return out, nil
}

func headerParsed(h map[string]int) bool { return len(h) > 0 }

// writeIssuedCSV materializes the one-time distribution table with private
// permissions; the plaintext keys live nowhere else on disk.
func writeIssuedCSV(path string, issued []service.UserKeyIssued) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return errors.New("invalid_user_batch")
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if e = w.Write([]string{"name", "user_id", "key", "admin", "expires_at"}); e != nil {
		return e
	}
	for _, v := range issued {
		expires := ""
		if !v.ExpiresAt.IsZero() {
			expires = v.ExpiresAt.Format("2006-01-02")
		}
		if e = w.Write([]string{v.Name, v.UserID, v.Key, fmt.Sprintf("%v", v.Admin), expires}); e != nil {
			return e
		}
	}
	w.Flush()
	return w.Error()
}
