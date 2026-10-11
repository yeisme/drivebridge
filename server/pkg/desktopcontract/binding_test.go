package desktopcontract

import (
	"encoding/json"
	"testing"
	"time"
)

func fixtureAs[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := json.Marshal(Fixtures()[name])
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func TestDesktopBindingPreviewAuthority(t *testing.T) {
	p := fixtureAs[BindingPreview](t, "BindingPreview")
	if p.Validate() != nil {
		t.Fatal("valid preview rejected")
	}
	for _, change := range []func(*BindingPreview){
		func(p *BindingPreview) { p.Scopes = []ReaderScope{ScopeBody} }, func(p *BindingPreview) { p.Scopes = append(p.Scopes, "writer.editor") },
		func(p *BindingPreview) { p.Scopes = append(p.Scopes, ScopeBody) }, func(p *BindingPreview) { p.ExpiresAt = p.ObservedAt.Add(6 * time.Minute) },
		func(p *BindingPreview) { p.ExpiresAt = p.ObservedAt }, func(p *BindingPreview) { p.PendingMutations = 1 }, func(p *BindingPreview) { p.Reason = "managed_scope_busy" },
		func(p *BindingPreview) { p.Decision = "unknown" }, func(p *BindingPreview) { p.ExpectedGuardRevision = maxSafeRevision + 1 },
	} {
		bad := p
		bad.Scopes = append([]ReaderScope{}, p.Scopes...)
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("unsafe preview accepted")
		}
	}
	p.Decision = "blocked"
	p.Reason = "managed_scope_unverifiable"
	if p.Validate() != nil {
		t.Fatal("blocked preview rejected")
	}
}
func TestDesktopBindingVisibilityAndRelease(t *testing.T) {
	b := fixtureAs[BindingView](t, "BindingView")
	for _, state := range []string{"active", "hidden", "blocked", "released"} {
		b.State = state
		b.ProtectionActive = state != "released"
		if b.Validate() != nil {
			t.Fatal("valid state rejected")
		}
		b.ProtectionActive = !b.ProtectionActive
		if b.Validate() == nil {
			t.Fatal("protection contradicted state")
		}
	}
	confirm := fixtureAs[ConfirmBinding](t, "ConfirmBinding")
	if confirm.Validate() != nil {
		t.Fatal("confirmation rejected")
	}
	confirm.Confirmed = false
	if confirm.Validate() == nil {
		t.Fatal("unconfirmed binding")
	}
	release := fixtureAs[ReleaseProtection](t, "ReleaseProtection")
	if release.Validate() != nil {
		t.Fatal("release rejected")
	}
	release.ExpectedRevision = 0
	if release.Validate() == nil {
		t.Fatal("unversioned release")
	}
	visibility := fixtureAs[SetSourceVisibility](t, "SetSourceVisibility")
	if visibility.Validate() != nil {
		t.Fatal("visibility rejected")
	}
	receipt := fixtureAs[BindingMutationReceipt](t, "BindingMutationReceipt")
	if receipt.Validate() != nil {
		t.Fatal("receipt rejected")
	}
	lookup := BindingLookupResult{Found: true, Receipt: &receipt}
	if lookup.Validate() != nil {
		t.Fatal("lookup rejected")
	}
	lookup.Found = false
	if lookup.Validate() == nil {
		t.Fatal("not found leaked receipt")
	}
	receipt.Action = "release"
	if receipt.Validate() == nil {
		t.Fatal("release receipt with active guard")
	}
}
func TestDesktopTopicIdentityAndLookup(t *testing.T) {
	topic := fixtureAs[UpdateTopicPin](t, "UpdateTopicPin")
	if topic.Validate() != nil {
		t.Fatal("topic rejected")
	}
	topic.TargetRef.Kind = "folder"
	if topic.Validate() == nil {
		t.Fatal("unapproved topic kind")
	}
	lookup := fixtureAs[BindingMutationLookup](t, "BindingMutationLookup")
	if lookup.Validate() != nil {
		t.Fatal("lookup rejected")
	}
	lookup.Action = "rotate"
	if lookup.Validate() == nil {
		t.Fatal("unknown lookup action")
	}
	lookup.Action = "confirm"
	lookup.IdempotencyKey = "../other"
	if lookup.Validate() == nil {
		t.Fatal("invalid recovery key")
	}
}
