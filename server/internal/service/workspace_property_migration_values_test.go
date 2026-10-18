package service

import (
	"encoding/json"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestPropertyMigrationConversionPolicies(t *testing.T) {
	for _, tc := range []struct{ name, source, target, policy, raw, want string }{
		{"rename exact numeric", "number", "number", "preserve", `1.250`, `1.250`},
		{"parse number exact", "text", "number", "parse_text", `"1.250"`, `1.250`},
		{"parse bool", "text", "checkbox", "parse_text", `"false"`, `false`},
		{"parse date", "text", "date", "parse_text", `"2024-02-29"`, `"2024-02-29"`},
		{"numeric text", "number", "text", "scalar_text", `1.250`, `"1.250"`},
		{"singleton", "text", "list", "singleton_list", `"a,b"`, `["a,b"]`},
		{"reject whitespace", "text", "number", "parse_text", `" 1 "`, ""},
		{"reject rounding", "text", "number", "parse_text", `"9007199254740992"`, ""},
		{"reject bool guessing", "text", "checkbox", "parse_text", `"yes"`, ""},
		{"reject invalid date", "text", "date", "parse_text", `"2025-02-29"`, ""},
		{"reject implicit parsing", "text", "number", "preserve", `"1"`, ""},
		{"reject list joining", "list", "text", "scalar_text", `["a","b"]`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertPropertyMigration(json.RawMessage(tc.raw), tc.source, contract.PropertyMigrationPreview{TargetType: tc.target, ConversionPolicy: tc.policy})
			if tc.want == "" {
				if err == nil {
					t.Fatalf("unexpected conversion %s", got)
				}
				return
			}
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}
