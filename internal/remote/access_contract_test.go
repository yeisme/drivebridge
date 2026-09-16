package remote

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAccessResourceAvailableWithoutNetworkOrInstalledCLI(t *testing.T) {
	a := &Adapter{unifiedIdentity: true}
	server := a.MCP()
	response := server.HandleMessage(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"drivebridge://service-access"}}`))
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"aigora_key_exchange", "service_admission_required", "operation_get", "client_cli_required"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing recovery field %s", want)
		}
	}
	if a.accessContract()["client_cli_required"] != false {
		t.Fatal("CLI required for connected client")
	}
}
