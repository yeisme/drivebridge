package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTeamWorkspaceStorageArchiveAndRestore(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	owner, _ := login(t, f, "owner")
	member, _ := login(t, f, "member")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	tenant := f.Service.Instance
	call := func(h *human, action string, q map[string]any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var team service.Team
	if st := call(root, "teams", map[string]any{"name": "Design", "user_id": owner.user.ID}, &team); st != 200 {
		t.Fatal("create", st)
	}
	latest := func() uint64 { var v service.Team; f.Service.DB.First(&v, "id = ?", team.ID); return v.Revision }
	change := func(h *human, action string, q map[string]any, out any) int {
		q["team_id"] = team.ID
		q["expected_revision"] = latest()
		return call(h, action, q, out)
	}
	if st := change(root, "team-member", map[string]any{"user_id": member.user.ID, "role": "member"}, nil); st != 200 {
		t.Fatal("member", st)
	}
	if st := call(root, "team-member", map[string]any{"team_id": team.ID, "user_id": owner.user.ID, "role": "remove"}, nil); st != 409 {
		t.Fatal("legacy route lost last admin guard", st)
	}
	if st := change(root, "team-member", map[string]any{"user_id": member.user.ID, "role": "admin"}, nil); st != 200 {
		t.Fatal(st)
	}
	oldRevision := latest()
	if st := change(root, "team-update", map[string]any{"name": "Design renamed", "description": "safe"}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := call(root, "team-update", map[string]any{"team_id": team.ID, "expected_revision": oldRevision, "name": "stale"}, nil); st != 409 {
		t.Fatal("stale team revision", st)
	}
	storage, e := f.Service.ConnectS3(context.Background(), service.Principal{UserID: root.user.ID, TenantID: tenant, DataOwner: root.space.Owner}, "Team S3", service.S3Config{Endpoint: f.S3.URL, Region: "us-east-1", Bucket: "fixture", Prefix: "team-extra", AccessKey: "fixture-key", SecretKey: "fixture-secret"})
	if e != nil {
		t.Fatal(e)
	}
	if st := change(member, "team-storage-assign", map[string]any{"connection_id": storage.ID}, nil); st != 403 {
		t.Fatal("team admin used private storage", st)
	}
	if st := change(root, "team-storage-assign", map[string]any{"connection_id": storage.ID}, nil); st != 200 {
		t.Fatal("assign", st)
	}
	var space service.Space
	if st := change(root, "spaces", map[string]any{"name": "Assets", "user_id": owner.user.ID, "connection_id": storage.ID}, &space); st != 200 {
		t.Fatal("space", st)
	}
	if st := change(root, "team-storage-unassign", map[string]any{"connection_id": storage.ID}, nil); st != 409 {
		t.Fatal("assigned connection removed", st)
	}
	if st := call(root, "permission", map[string]any{"space_id": space.ID, "user_id": owner.user.ID, "role": "viewer"}, nil); st != 409 {
		t.Fatal("legacy route orphaned custodian", st)
	}
	if st := change(root, "team-member", map[string]any{"user_id": owner.user.ID, "role": "remove"}, nil); st != 409 {
		t.Fatal("member removal orphaned custodian", st)
	}
	if st := change(root, "permission", map[string]any{"space_id": space.ID, "user_id": member.user.ID, "role": "viewer"}, nil); st != 200 {
		t.Fatal(st)
	}
	f.Service.DB.First(&space, "id = ?", space.ID)
	p := service.Principal{UserID: owner.user.ID, TenantID: tenant, DataOwner: space.Owner, ScopeSpace: space.ID}
	upload := func(name string, complete bool) service.Operation {
		t.Helper()
		body := []byte("team material " + name)
		h := sha256.Sum256(body)
		op, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: space.ID, Path: name, Size: int64(len(body)), SHA256: hex.EncodeToString(h[:])}, service.ID())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Service.PutPart(p, op.ID, 0, bytes.NewReader(body)); e != nil {
			t.Fatal(e)
		}
		if complete {
			op, e = f.Service.Complete(context.Background(), p, op.ID)
			if e != nil {
				t.Fatal(e)
			}
		}
		return op
	}
	complete := upload("sample.txt", true)
	pending := upload("pending.txt", false)
	if st := govCall(t, root, tenant, space.ID, "POST", "/api/v1/directories", map[string]string{"space_id": space.ID, "path": "forbidden"}, nil); st != 403 {
		t.Fatal("admin edited without grant", st)
	}
	var access struct {
		URL       string `json:"url"`
		Transport string `json:"transport"`
	}
	if st := govCall(t, owner, tenant, space.ID, "POST", "/api/v1/files/"+complete.FileID+"/access", map[string]string{"version_id": complete.VersionID}, &access); st != 200 || !strings.Contains(access.URL, "/api/v1/access/") {
		t.Fatal("team emitted unrevocable storage link", st)
	}
	var share struct {
		Share service.Share `json:"share"`
		Link  string        `json:"link"`
	}
	if st := govCall(t, owner, tenant, space.ID, "POST", "/api/v1/shares", map[string]any{"name": "External", "space_id": space.ID, "mode": "fixed", "audience": "link", "days": 1, "allow_download": true, "items": []map[string]string{{"file_id": complete.FileID, "version_id": complete.VersionID}}}, &share); st != 200 {
		t.Fatal("share", st)
	}
	agent := service.Agent{ID: service.ID(), Owner: space.Owner, SpaceID: space.ID, TokenHash: service.Hash(service.ID()), Name: "Team agent", Scopes: "read", Expires: time.Now().Add(time.Hour)}
	f.Service.DB.Create(&agent)
	if st := change(root, "team-archive", map[string]any{}, nil); st != 200 {
		t.Fatal("archive", st)
	}
	if _, e = f.Service.Complete(context.Background(), p, pending.ID); e == nil {
		t.Fatal("archived pending upload committed")
	}
	if _, e = f.Service.Cancel(p, pending.ID); e != nil {
		t.Fatal("cannot release archived pending upload", e)
	}
	if e = f.Service.Trash(p, complete.FileID, false); e == nil {
		t.Fatal("archived write allowed")
	}
	if _, _, body, e := f.Service.Download(context.Background(), p, complete.FileID, complete.VersionID); e != nil {
		t.Fatal("archive removed prior read", e)
	} else {
		b, _ := io.ReadAll(body)
		body.Close()
		if string(b) != "team material sample.txt" {
			t.Fatal("wrong shared storage bytes")
		}
	}
	res, e := http.Get(access.URL)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("archived external grant survived")
	}
	f.Service.DB.First(&share.Share, "id = ?", share.Share.ID)
	if !share.Share.Revoked {
		t.Fatal("share not revoked")
	}
	f.Service.DB.First(&agent, "id = ?", agent.ID)
	if !agent.Revoked || !agent.ArchiveSuspended {
		t.Fatal("agent not suspended")
	}
	if st := change(owner, "team-restore", map[string]any{}, nil); st != 403 {
		t.Fatal("team admin restored without tenant authority", st)
	}
	if st := change(root, "team-restore", map[string]any{}, nil); st != 200 {
		t.Fatal("restore", st)
	}
	f.Service.DB.First(&share.Share, "id = ?", share.Share.ID)
	if !share.Share.Revoked {
		t.Fatal("restore resurrected share")
	}
	f.Service.DB.First(&agent, "id = ?", agent.ID)
	if !agent.Revoked {
		t.Fatal("restore resurrected agent")
	}
	var detail map[string]any
	if st := govCall(t, member, tenant, "", "GET", "/api/v1/governance/teams/"+team.ID, nil, &detail); st != 200 {
		t.Fatal("detail", st)
	}
	// A fabricated space owner cannot use an assignment belonging to this team.
	stranger := service.Principal{UserID: member.user.ID, TenantID: tenant, DataOwner: member.space.Owner}
	if _, _, body, e := f.Service.Download(context.Background(), stranger, complete.FileID, complete.VersionID); e == nil {
		body.Close()
		t.Fatal("cross-owner storage read")
	}
}

func TestTeamInvitationsPaginationAndLegacyIsolation(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	member, _ := login(t, f, "member")
	other, _ := login(t, f, "other")
	f.Service.BootstrapGovernance(root.user.ID)
	tenant := f.Service.Instance
	call := func(h *human, action string, q map[string]any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var team service.Team
	call(root, "teams", map[string]any{"name": "Invitations", "user_id": root.user.ID}, &team)
	q := map[string]any{"team_id": team.ID, "expected_revision": team.Revision, "user_id": member.user.ID}
	var invited struct {
		Team   service.Team           `json:"team"`
		Result service.TeamInvitation `json:"result"`
	}
	if st := call(root, "team-invite", q, &invited); st != 200 {
		t.Fatal(st)
	}
	if st := call(other, "team-invite-accept", map[string]any{"id": invited.Result.ID, "expected_revision": invited.Team.Revision}, nil); st == 200 {
		t.Fatal("other user accepted")
	}
	if st := call(member, "team-invite-accept", map[string]any{"id": invited.Result.ID, "expected_revision": invited.Team.Revision}, nil); st != 200 {
		t.Fatal("accept", st)
	}
	var count int64
	f.Service.DB.Model(&service.SpacePermission{}).Where("user_id = ?", member.user.ID).Count(&count)
	if count != 0 {
		t.Fatal("invitation silently granted file permissions")
	}
	var rows struct {
		Items []any `json:"items"`
		Next  int   `json:"next_page"`
	}
	if st := govCall(t, member, tenant, "", "GET", "/api/v1/governance/teams?limit=1&filter=joined", nil, &rows); st != 200 || len(rows.Items) != 1 {
		t.Fatal("joined page", st)
	}
	if st := govCall(t, other, tenant, "", "GET", "/api/v1/governance/teams/"+team.ID, nil, nil); st != 404 {
		t.Fatal("team detail leaked", st)
	}
	f.Service.DB.First(&team, "id = ?", team.ID)
	var expired struct {
		Result service.TeamInvitation `json:"result"`
	}
	if st := call(root, "team-invite", map[string]any{"team_id": team.ID, "expected_revision": team.Revision, "user_id": other.user.ID}, &expired); st != 200 {
		t.Fatal(st)
	}
	f.Service.DB.Model(&service.TeamInvitation{}).Where("id = ?", expired.Result.ID).Update("expires", time.Now().Add(-time.Minute))
	var history struct {
		Items []service.TeamInvitation `json:"items"`
	}
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/governance/teams/"+team.ID+"/invitations", nil, &history); st != 200 {
		t.Fatal(st)
	}
	found := false
	for _, row := range history.Items {
		if row.ID == expired.Result.ID && row.Status == "expired" {
			found = true
		}
	}
	if !found {
		t.Fatal("API presented expired invitation as pending")
	}
	var t2 service.Team
	call(root, "teams", map[string]any{"name": "Second", "user_id": root.user.ID}, &t2)
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/governance/teams?limit=1", nil, &rows); st != 200 || rows.Next != 2 {
		t.Fatal("pagination", st, rows.Next)
	}
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/governance/teams?limit=1001", nil, nil); st == 200 {
		t.Fatal("unbounded limit accepted")
	}
}

func TestTeamInvitationDeclineAndResend(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	member, _ := login(t, f, "member")
	other, _ := login(t, f, "other")
	f.Service.BootstrapGovernance(root.user.ID)
	tenant := f.Service.Instance
	call := func(h *human, action string, q map[string]any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var team service.Team
	call(root, "teams", map[string]any{"name": "Lifecycle", "user_id": root.user.ID}, &team)
	rev := func() uint64 {
		var v service.Team
		f.Service.DB.First(&v, "id = ?", team.ID)
		return v.Revision
	}
	var invited struct {
		Team   service.Team           `json:"team"`
		Result service.TeamInvitation `json:"result"`
	}
	if st := call(root, "team-invite", map[string]any{"team_id": team.ID, "expected_revision": rev(), "user_id": member.user.ID}, &invited); st != 200 {
		t.Fatal("invite", st)
	}
	if st := call(other, "team-invite-decline", map[string]any{"id": invited.Result.ID, "expected_revision": rev()}, nil); st == 200 {
		t.Fatal("other declined")
	}
	if st := call(member, "team-invite-decline", map[string]any{"id": invited.Result.ID, "expected_revision": rev()}, nil); st != 200 {
		t.Fatal("decline", st)
	}
	var stored service.TeamInvitation
	f.Service.DB.First(&stored, "id = ?", invited.Result.ID)
	if stored.Status != "declined" {
		t.Fatalf("status %s", stored.Status)
	}
	var members int64
	f.Service.DB.Model(&service.TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, member.user.ID).Count(&members)
	if members != 0 {
		t.Fatal("decline created membership")
	}
	if st := call(root, "team-invite-resend", map[string]any{"id": invited.Result.ID, "expected_revision": rev()}, nil); st != 409 {
		t.Fatal("resend declined", st)
	}
	var again struct {
		Result service.TeamInvitation `json:"result"`
	}
	if st := call(root, "team-invite", map[string]any{"team_id": team.ID, "expected_revision": rev(), "user_id": member.user.ID}, &again); st != 200 {
		t.Fatal("invite again", st)
	}
	id := again.Result.ID
	f.Service.DB.Model(&service.TeamInvitation{}).Where("id = ?", id).Update("expires", time.Now().Add(-time.Minute))
	if st := call(member, "team-invite-decline", map[string]any{"id": id, "expected_revision": rev()}, nil); st != 409 {
		t.Fatal("decline expired", st)
	}
	var renewed struct {
		Result service.TeamInvitation `json:"result"`
	}
	if st := call(root, "team-invite-resend", map[string]any{"id": id, "expected_revision": rev()}, &renewed); st != 200 {
		t.Fatal("resend", st)
	}
	if renewed.Result.ID != id {
		t.Fatal("resend created a new invitation")
	}
	if !renewed.Result.Expires.After(time.Now()) {
		t.Fatal("resend did not extend expiry")
	}
	var inbox struct {
		Items []struct {
			Status   string `json:"status"`
			NextStep string `json:"next_step"`
		} `json:"items"`
	}
	if st := govCall(t, member, tenant, "", "GET", "/api/v1/governance/team-invitations", nil, &inbox); st != 200 {
		t.Fatal("inbox", st)
	}
	found := false
	for _, row := range inbox.Items {
		if row.Status == "pending" && row.NextStep == "accept_team" {
			found = true
		}
	}
	if !found {
		t.Fatalf("inbox %#v", inbox.Items)
	}
	if st := call(member, "team-invite-accept", map[string]any{"id": id, "expected_revision": rev()}, nil); st != 200 {
		t.Fatal("accept after resend", st)
	}
	if st := call(root, "team-invite-resend", map[string]any{"id": id, "expected_revision": rev()}, nil); st != 409 {
		t.Fatal("resend accepted", st)
	}
	var perms int64
	f.Service.DB.Model(&service.SpacePermission{}).Where("user_id = ?", member.user.ID).Count(&perms)
	if perms != 0 {
		t.Fatal("invitation granted file permissions")
	}
}

func TestTeamInvitationTerminalNextStep(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	member, _ := login(t, f, "member")
	other, _ := login(t, f, "other")
	f.Service.BootstrapGovernance(root.user.ID)
	tenant := f.Service.Instance
	call := func(h *human, action string, q map[string]any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var team service.Team
	call(root, "teams", map[string]any{"name": "Terminal", "user_id": root.user.ID}, &team)
	rev := func() uint64 {
		var v service.Team
		f.Service.DB.First(&v, "id = ?", team.ID)
		return v.Revision
	}
	invite := func(user *human) string {
		var out struct {
			Result service.TeamInvitation `json:"result"`
		}
		if st := call(root, "team-invite", map[string]any{"team_id": team.ID, "expected_revision": rev(), "user_id": user.user.ID}, &out); st != 200 {
			t.Fatal("invite", st)
		}
		return out.Result.ID
	}
	accepted := invite(member)
	if st := call(member, "team-invite-accept", map[string]any{"id": accepted, "expected_revision": rev()}, nil); st != 200 {
		t.Fatal("accept", st)
	}
	declined := invite(other)
	if st := call(other, "team-invite-decline", map[string]any{"id": declined, "expected_revision": rev()}, nil); st != 200 {
		t.Fatal("decline", st)
	}
	var history struct {
		Items []struct {
			Status   string `json:"status"`
			NextStep string `json:"next_step"`
		} `json:"items"`
	}
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/governance/teams/"+team.ID+"/invitations", nil, &history); st != 200 {
		t.Fatal("history", st)
	}
	for _, row := range history.Items {
		if row.Status == "accepted" || row.Status == "declined" || row.Status == "cancelled" {
			if row.NextStep != "none" {
				t.Fatalf("%s invitation offered next_step %q", row.Status, row.NextStep)
			}
		}
	}
}
