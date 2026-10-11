package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCommittedFixturesMatchGoSource(t *testing.T) {
	expected, err := EncodeResponseFixtures()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "personal-workspace", "fixtures", "responses.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(actual), expected) {
		t.Fatal("generated fixture drift: run make contracts")
	}
	var fixtures map[string]json.RawMessage
	if err = json.Unmarshal(actual, &fixtures); err != nil {
		t.Fatal(err)
	}
	var replay DataEnvelope[GrantReplayView]
	if err = json.Unmarshal(fixtures["GrantReplayEnvelope"], &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Data.Credential.State != "not_replayable" || replay.Data.Grant.ExpiresAt != nil {
		t.Fatal("replay contract changed")
	}
}
