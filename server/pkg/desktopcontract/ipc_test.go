package desktopcontract

import (
	"strings"
	"testing"
)

func TestDesktopIPCMethodsAreFixedAndPlanned(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range IPCMethods() {
		if seen[m.Name] || m.Status != "planned" || !strings.HasPrefix(m.Path, "/v1/desktop/") {
			t.Fatal("invalid method", m.Name)
		}
		seen[m.Name] = true
		if !strings.HasPrefix(m.RequestSchema, "pinax.") && Schemas()[m.RequestSchema] == nil {
			t.Fatal("missing input", m.Name)
		}
		if !strings.HasPrefix(m.ResponseSchema, "pinax.") && Schemas()[m.ResponseSchema] == nil {
			t.Fatal("missing output", m.Name)
		}
		switch m.Name {
		case "confirmBinding", "setSourceVisibility", "releaseProtection", "updateReadingState", "updateTopicPin", "exportSource":
			if !m.Idempotent {
				t.Fatal("mutation lacks original key", m.Name)
			}
		}
		if m.Name == "chooseVault" && (!m.NativeSelection || m.RequestSchema != "EmptyRequest") {
			t.Fatal("chooser contract")
		}
		if m.Name == "exportSource" && (!m.NativeSelection || m.RequestSchema != "ExportSourceInput") {
			t.Fatal("export accepts private input")
		}
	}
	if len(seen) != 23 {
		t.Fatal("incomplete catalog")
	}
	if _, err := Encode("ipc"); err != nil {
		t.Fatal(err)
	}
}
