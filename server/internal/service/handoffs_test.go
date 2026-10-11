package service_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestPinnedHandoffLifecycle(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	_, other := agent(t, h, "", "read,write")
	ctx := context.Background()
	data := []byte("fixed file contents")
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/source.txt"}, "source", bytes.NewReader(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	caps := map[string]any{}
	if err := c.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &caps); err != nil {
		t.Fatal(err)
	}
	instance, _ := caps["instance_id"].(string)
	if status := h.call(t, "POST", "/handoff-bindings", sdk.HandoffBinding{SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "project-a", Purpose: "reference"}, nil); status == 200 {
		t.Fatal("missing agent selected an arbitrary agent")
	}
	var b sdk.HandoffBinding
	if status := h.call(t, "POST", "/handoff-bindings", sdk.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "project-a", Purpose: "reference"}, &b); status != 200 {
		t.Fatal("binding", status)
	}
	expires := time.Now().Add(time.Hour).UTC()
	q := sdk.BindReferenceRequest{BindingID: b.ID, Ref: "drivebridge://" + instance + "/file/" + op.FileID + "/version/" + op.VersionID, Consumer: "eikona", ProjectRef: "project-a", Purpose: "reference", ExpectedSHA256: op.SHA256, ConsumerRequestRef: "import-one", RequestExpiresAt: &expires}
	g, err := c.BindReference(ctx, q, "same-input")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			again, err := c.BindReference(ctx, q, "same-input")
			if err != nil || again.ID != g.ID {
				t.Errorf("replay: %v %s", err, again.ID)
			}
		}()
	}
	wg.Wait()
	changed := q
	changed.ConsumerRequestRef = "another-input"
	if _, err := c.BindReference(ctx, changed, "same-input"); err == nil {
		t.Fatal("different fingerprint accepted")
	}
	if _, err := other.OpenHandoff(ctx, g.ID); err == nil {
		t.Fatal("unbound receiver allowed")
	}
	body, err := c.OpenHandoff(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("bytes", err)
	}
	newer, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/source.txt", FileID: op.FileID, ExpectedVersion: op.VersionID}, "replacement", bytes.NewReader([]byte("new contents")), nil)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := c.OpenHandoff(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldBytes, err := io.ReadAll(pinned)
	pinned.Close()
	if err != nil || !bytes.Equal(oldBytes, data) {
		t.Fatal("pinned version followed replacement", err)
	}
	for _, target := range []string{"outside/source.txt", "refs/source.txt"} {
		if status := h.call(t, "POST", "/files/"+op.FileID+"/move", map[string]string{"path": target, "expected_version": newer.VersionID}, nil); status != 200 {
			t.Fatal("move", status)
		}
		stream, err := c.OpenHandoff(ctx, g.ID)
		if target == "outside/source.txt" && err == nil {
			stream.Close()
			t.Fatal("moved file escaped binding")
		}
		if target == "refs/source.txt" {
			if err != nil {
				t.Fatal(err)
			}
			stream.Close()
		}
	}
	if _, err := c.RenewHandoff(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CancelHandoff(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenHandoff(ctx, g.ID); err == nil {
		t.Fatal("cancelled read allowed")
	}
	if _, err := c.RenewHandoff(ctx, g.ID); err == nil {
		t.Fatal("cancelled request revived")
	}
	second, err := c.BindReference(ctx, q, "another-operation")
	if err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "POST", "/handoff-bindings/"+b.ID+"/disable", struct{}{}, nil); status != 200 {
		t.Fatal(status)
	}
	if _, err := c.OpenHandoff(ctx, second.ID); err == nil {
		t.Fatal("disabled binding read allowed")
	}
}

func TestHandoffSurvivesServiceRestart(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	ctx := context.Background()
	data := []byte("restart contents")
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/restart.txt"}, "source", bytes.NewReader(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := service.Principal{UserID: h.user.ID}
	binding, err := f.Service.CreateHandoffBinding(owner, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "reference"})
	if err != nil {
		t.Fatal(err)
	}
	var caps map[string]any
	if err := c.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &caps); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC()
	q := sdk.BindReferenceRequest{BindingID: binding.ID, Ref: "drivebridge://" + caps["instance_id"].(string) + "/file/" + op.FileID + "/version/" + op.VersionID, Consumer: "eikona", ProjectRef: "p", Purpose: "reference", ExpectedSHA256: op.SHA256, ConsumerRequestRef: "original-input", RequestExpiresAt: &expires}
	original, err := c.BindReference(ctx, q, "original-key")
	if err != nil {
		t.Fatal(err)
	}
	config := f.Service.Config
	f.Service.Close()
	reopened, err := service.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	f.Service = reopened
	receiver := service.Principal{UserID: h.user.ID, Agent: &service.Agent{ID: a.ID}}
	recovered, err := reopened.ChangeHandoff(receiver, original.ID, "renew")
	if err != nil || recovered.ID != original.ID || recovered.RequestRef != "original-input" {
		t.Fatalf("handoff changed after restart: %+v %v", recovered, err)
	}
	_, body, err := reopened.OpenHandoff(ctx, receiver, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("restart bytes", err)
	}
}

func TestHandoffMCPBoundLifecycle(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	ctx := context.Background()
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/mcp.txt"}, "source", bytes.NewReader([]byte("mcp reference")), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Service.CreateHandoffBinding(service.Principal{UserID: h.user.ID}, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "reference"})
	if err != nil {
		t.Fatal(err)
	}
	p := service.Principal{UserID: h.user.ID, Agent: &service.Agent{ID: a.ID}}
	args := map[string]any{"binding_id": b.ID, "ref": "drivebridge://" + f.Service.Instance + "/file/" + op.FileID + "/version/" + op.VersionID, "consumer": "eikona", "project_ref": "p", "purpose": "reference", "expected_sha256": op.SHA256, "idempotency_key": "mcp-original", "consumer_request_ref": "input-original", "request_expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
	value, err := f.Service.CallTool(ctx, p, "input.bind_reference", args)
	if err != nil {
		t.Fatal(err)
	}
	bound := value.(map[string]any)
	id := bound["grant_id"].(string)
	if bound["binding_id"] != b.ID {
		t.Fatal("MCP ignored receiver binding")
	}
	for _, action := range []string{"input.handoff_status", "input.handoff_renew", "input.handoff_cancel"} {
		result, err := f.Service.CallTool(ctx, p, action, map[string]any{"grant_id": id})
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if action == "input.handoff_cancel" && result.(map[string]any)["state"] != "cancelled" {
			t.Fatal("cancel state missing")
		}
	}
	if _, err := f.Service.CallTool(ctx, p, "input.handoff_renew", map[string]any{"grant_id": id}); err == nil {
		t.Fatal("cancelled grant renewed")
	}
	args["request_expires_at"] = "invalid"
	if _, err := f.Service.CallTool(ctx, p, "input.bind_reference", args); err == nil {
		t.Fatal("invalid expiry ignored")
	}
}

func TestHandoffBindingPurposeAllowlist(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "", "read,write")
	owner := service.Principal{UserID: h.user.ID}
	if len(sdk.AllowedHandoffPurposes) == 0 || sdk.AllowedHandoffPurposes[0] != "reference" {
		t.Fatal("reference must remain first-class")
	}
	for _, purpose := range sdk.AllowedHandoffPurposes {
		got, err := f.Service.CreateHandoffBinding(owner, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p-" + purpose, Purpose: purpose})
		if err != nil || got.Purpose != purpose || got.Disabled {
			t.Fatalf("purpose %s: %+v %v", purpose, got, err)
		}
	}
	if _, err := f.Service.CreateHandoffBinding(owner, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "drama-ref"}); err == nil || err.Error() != "invalid_binding" {
		t.Fatal("unknown purpose accepted", err)
	}
	if status := h.call(t, "POST", "/handoff-bindings", sdk.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "http-unknown", Purpose: "unknown"}, nil); status != 400 {
		t.Fatal("unknown purpose status", status)
	}
}

func TestHandoffBindingCreateReusesSameScope(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "", "read,write")
	owner := service.Principal{UserID: h.user.ID}
	q := service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "pinax", ProjectRef: "drama-1", Purpose: "materials"}
	first, err := f.Service.CreateHandoffBinding(owner, q)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.Service.CreateHandoffBinding(owner, q)
	if err != nil || again.ID != first.ID {
		t.Fatalf("same scope was not reused: %+v %v", again, err)
	}
	q.Prefix = "other"
	other, err := f.Service.CreateHandoffBinding(owner, q)
	if err != nil || other.ID == first.ID {
		t.Fatalf("different prefix reused id: %+v %v", other, err)
	}
}

func TestHandoffPrefixRecheckOnRead(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	ctx := context.Background()
	data := []byte("prefix-fence")
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/source.txt"}, "source", bytes.NewReader(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Service.CreateHandoffBinding(service.Principal{UserID: h.user.ID}, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "materials"})
	if err != nil {
		t.Fatal(err)
	}
	caps := map[string]any{}
	if err := c.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &caps); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC()
	g, err := c.BindReference(ctx, sdk.BindReferenceRequest{BindingID: b.ID, Ref: "drivebridge://" + caps["instance_id"].(string) + "/file/" + op.FileID + "/version/" + op.VersionID, Consumer: "eikona", ProjectRef: "p", Purpose: "materials", ExpectedSHA256: op.SHA256, ConsumerRequestRef: "prefix-one", RequestExpiresAt: &expires}, "prefix-key")
	if err != nil {
		t.Fatal(err)
	}
	if status := h.call(t, "POST", "/files/"+op.FileID+"/move", map[string]string{"path": "outside/source.txt", "expected_version": op.VersionID}, nil); status != 200 {
		t.Fatal("move", status)
	}
	if _, err := c.GetHandoff(ctx, g.ID); err == nil || err.Error() != "forbidden" {
		t.Fatal("GetHandoff after move", err)
	}
	if body, err := c.OpenHandoff(ctx, g.ID); err == nil {
		body.Close()
		t.Fatal("OpenHandoff after move")
	} else if err.Error() != "forbidden" {
		t.Fatal("OpenHandoff code", err)
	}
	if status := h.call(t, "POST", "/files/"+op.FileID+"/move", map[string]string{"path": "refs/source.txt", "expected_version": op.VersionID}, nil); status != 200 {
		t.Fatal("move back", status)
	}
	body, err := c.OpenHandoff(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("bytes after return", err)
	}
}

func TestHandoffBindingDisableStopsReads(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	ctx := context.Background()
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/keep.txt"}, "source", bytes.NewReader([]byte("keep")), nil)
	if err != nil {
		t.Fatal(err)
	}
	var b sdk.HandoffBinding
	if status := h.call(t, "POST", "/handoff-bindings", sdk.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "saveback"}, &b); status != 200 {
		t.Fatal("binding", status)
	}
	caps := map[string]any{}
	if err := c.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &caps); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC()
	q := sdk.BindReferenceRequest{BindingID: b.ID, Ref: "drivebridge://" + caps["instance_id"].(string) + "/file/" + op.FileID + "/version/" + op.VersionID, Consumer: "eikona", ProjectRef: "p", Purpose: "saveback", ExpectedSHA256: op.SHA256, ConsumerRequestRef: "keep-one", RequestExpiresAt: &expires}
	g, err := c.BindReference(ctx, q, "disable-key")
	if err != nil {
		t.Fatal(err)
	}
	var disabled map[string]any
	if status := h.call(t, "POST", "/handoff-bindings/"+b.ID+"/disable", struct{}{}, &disabled); status != 200 || disabled["disabled"] != true {
		t.Fatal("disable", status, disabled)
	}
	for _, key := range []string{"deleted", "purged", "removed"} {
		if _, ok := disabled[key]; ok {
			t.Fatal("disable claimed copies deleted")
		}
	}
	if _, err := c.BindReference(ctx, q, "disable-after"); err == nil || err.Error() != "forbidden" {
		t.Fatal("BindReference after disable", err)
	}
	if body, err := c.OpenHandoff(ctx, g.ID); err == nil {
		body.Close()
		t.Fatal("OpenHandoff after disable")
	} else if err.Error() != "forbidden" {
		t.Fatal("OpenHandoff code", err)
	}
	var st struct {
		File service.File `json:"file"`
	}
	if status := h.call(t, "GET", "/files/"+op.FileID, nil, &st); status != 200 || st.File.ID != op.FileID || st.File.Trashed {
		t.Fatal("source file missing after disable", status, st.File)
	}
	var out bytes.Buffer
	if err := c.Download(ctx, op.FileID, "", &out); err != nil || out.String() != "keep" {
		t.Fatal("imported source bytes missing after disable", err)
	}
}

func TestHandoffCallerLabelsAreNotAuth(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	_, otherC := agent(t, h, "", "read,write")
	ctx := context.Background()
	if err := c.JSON(ctx, "POST", "/api/v1/handoff-bindings", "", sdk.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "vault"}, &sdk.HandoffBinding{}); err == nil || err.Error() != "forbidden" {
		t.Fatal("agent created binding", err)
	}
	if _, err := f.Service.CreateHandoffBinding(service.Principal{UserID: h.user.ID, Agent: &service.Agent{ID: a.ID}}, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "vault"}); err == nil || err.Error() != "forbidden" {
		t.Fatal("agent principal created binding", err)
	}
	b, err := f.Service.CreateHandoffBinding(service.Principal{UserID: h.user.ID}, service.HandoffBinding{AgentID: a.ID, SpaceID: h.space.ID, Prefix: "refs", Consumer: "eikona", ProjectRef: "p", Purpose: "vault"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/label.txt"}, "source", bytes.NewReader([]byte("label")), nil)
	if err != nil {
		t.Fatal(err)
	}
	caps := map[string]any{}
	if err := c.JSON(ctx, "GET", "/api/v1/capabilities", "", nil, &caps); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC()
	ref := "drivebridge://" + caps["instance_id"].(string) + "/file/" + op.FileID + "/version/" + op.VersionID
	q := sdk.BindReferenceRequest{BindingID: b.ID, Ref: ref, Consumer: "eikona", ProjectRef: "p", Purpose: "vault", ExpectedSHA256: op.SHA256, ConsumerRequestRef: "label-one", RequestExpiresAt: &expires}
	if _, err := otherC.BindReference(ctx, q, "other-agent"); err == nil || err.Error() != "forbidden" {
		t.Fatal("other agent bound with same consumer", err)
	}
	wrong := q
	wrong.Consumer = "scaena"
	if _, err := c.BindReference(ctx, wrong, "wrong-consumer"); err == nil || err.Error() != "forbidden" {
		t.Fatal("consumer label overrode binding", err)
	}
	g, err := c.BindReference(ctx, q, "owner-agent")
	if err != nil {
		t.Fatal(err)
	}
	if body, err := otherC.OpenHandoff(ctx, g.ID); err == nil {
		body.Close()
		t.Fatal("other agent read with same consumer")
	} else if err.Error() != "forbidden" {
		t.Fatal("OpenHandoff code", err)
	}
}
