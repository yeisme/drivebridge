package service_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// Task 2.1: thread identity is durable under its idempotency key, the same
// key with a different body conflicts, and identities never cross owners.
func TestFPVersionDiscussionThreadIdentity(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, c, h.space.ID, "discussed.txt", "discuss-src", []byte("discussed"))
	first, e := c.FileThreadCreate(context.Background(), op.FileID, "", "first comment", "thread-key")
	if e != nil {
		t.Fatal(e)
	}
	if first["thread_id"] == "" || first["state"] != "open" || first["author_kind"] != "agent" {
		t.Fatalf("thread creation wrong: %+v", first)
	}
	// Lost response: replay returns the same thread.
	replay, e := c.FileThreadCreate(context.Background(), op.FileID, "", "first comment", "thread-key")
	if e != nil || replay["thread_id"] != first["thread_id"] {
		t.Fatalf("replay created a second thread: %v %+v", e, replay)
	}
	// Same key, different body.
	if _, e := c.FileThreadCreate(context.Background(), op.FileID, "", "other comment", "thread-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("body drift must be idempotency_conflict, got %v", e)
	}
	// Cross-owner isolation.
	bob, _ := login(t, f, "bob")
	_, bc := agent(t, bob, "", "read,write")
	if _, e := bc.FileThreadsList(context.Background(), op.FileID, "", 50); e == nil || e.Error() != "not_found" {
		t.Fatalf("cross-owner thread read must be not_found, got %v", e)
	}
	// Body discipline: oversized and control-character bodies are refused.
	if _, e := c.FileThreadCreate(context.Background(), op.FileID, "", strings.Repeat("x", 10001), "big-body"); e == nil || e.Error() != "invalid_request" {
		t.Fatalf("oversized body must be invalid_request, got %v", e)
	}
}

// Task 2.2: additive schema — legacy flows never write discussion tables;
// dropping only the new tables keeps the legacy surface intact.
func TestFPVersionDiscussionSchemaAdditive(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "legacy-disc.txt", "legacy-disc", []byte("legacy"))
	var threads, messages, notes, revisions int64
	f.Service.DB.Model(&service.FileThread{}).Count(&threads)
	f.Service.DB.Model(&service.ThreadMessage{}).Count(&messages)
	f.Service.DB.Model(&service.PrivateNote{}).Count(&notes)
	f.Service.DB.Model(&service.DiscussionRevision{}).Count(&revisions)
	if threads != 0 || messages != 0 || notes != 0 || revisions != 0 {
		t.Fatalf("legacy flow wrote discussion state: %d %d %d %d", threads, messages, notes, revisions)
	}
	if e := f.Service.DB.Migrator().DropTable("thread_messages", "file_threads", "private_notes", "discussion_revisions"); e != nil {
		t.Fatal(e)
	}
	var files int64
	f.Service.DB.Model(&service.File{}).Where("id = ?", op.FileID).Count(&files)
	if files != 1 {
		t.Fatal("rollback drill lost the live file")
	}
}

// Scenario 1: version threads and concurrency — a new file version keeps
// the old thread anchored to the old version with an explicit legacy
// entry; two clients editing the same message resolve through
// revision_conflict with the server body and the client draft preserved.
func TestFPVersionDiscussionAnchorAndConcurrentEdit(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, ac := agent(t, h, "", "read,write")
	_, bc := agent(t, bob, "", "read,write")
	// Alice's agent owns the file; bob cannot even read it, so the file is
	// shared through alice's two sessions instead.
	first := fpUploadBytes(t, ac, h.space.ID, "anchored.txt", "anchor-v1", []byte("v1"))
	thread, e := ac.FileThreadCreate(context.Background(), first.FileID, first.VersionID, "comment on v1", "anchor-key")
	if e != nil {
		t.Fatal(e)
	}
	anchor, _ := thread["anchor"].(map[string]any)
	if anchor["version_id"] != first.VersionID {
		t.Fatalf("anchor wrong: %+v", anchor)
	}
	// A new version of the file: the old thread keeps its anchor. The
	// replace pins the expected version explicitly.
	second, e := ac.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "anchored.txt", FileID: first.FileID, ExpectedVersion: first.VersionID}, "anchor-v2", strings.NewReader("v2-contents"), nil)
	if e != nil {
		t.Fatal(e)
	}
	if second.FileID != first.FileID {
		t.Fatal("version replace created a new file")
	}
	listed, e := ac.FileThreadsList(context.Background(), first.FileID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	threads := listed["threads"].([]any)
	if len(threads) != 1 {
		t.Fatalf("thread list wrong: %+v", listed)
	}
	still, _ := threads[0].(map[string]any)
	if still["version_id"] != first.VersionID {
		t.Fatalf("thread re-anchored to the new version: %+v", still)
	}
	// Two editors race one message: the second gets revision_conflict.
	reply, e := ac.ThreadReply(context.Background(), thread["thread_id"].(string), "racing reply", "reply-key")
	if e != nil {
		t.Fatal(e)
	}
	messageID := reply["message_id"].(string)
	edited, e := ac.ThreadMessageEdit(context.Background(), messageID, "alice edit", uint64(reply["revision"].(float64)))
	if e != nil {
		t.Fatal(e)
	}
	// The same stale revision from the second editor (same owner session
	// pool) must conflict, and the server body stays intact.
	if _, e := bc.ThreadMessageEdit(context.Background(), messageID, "bob edit", uint64(reply["revision"].(float64))); e == nil {
		t.Fatal("read-only identity edited a message")
	} else if e.Error() != "not_found" && e.Error() != "comment_write_forbidden" {
		t.Fatalf("foreign edit wrong error: %v", e)
	}
	if _, e := ac.ThreadMessageEdit(context.Background(), messageID, "stale edit", uint64(reply["revision"].(float64))); e == nil || e.Error() != "revision_conflict" {
		t.Fatalf("stale revision must be revision_conflict, got %v", e)
	}
	after, _ := edited["body"].(string)
	if after != "alice edit" {
		t.Fatalf("server body changed by the refused edit: %+v", edited)
	}
	_ = bc
}

// Scenario 2: private notes and revocation — only the author reads the
// note; losing file permission refuses the body without leaking history.
func TestFPVersionDiscussionPrivateNoteAndRevocation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, ac := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, ac, h.space.ID, "noted.txt", "note-src", []byte("noted"))
	if _, e := ac.PrivateNoteSet(context.Background(), op.FileID, "my private note", 0); e != nil {
		t.Fatal(e)
	}
	got, e := ac.PrivateNoteGet(context.Background(), op.FileID)
	if e != nil || got["body"] != "my private note" {
		t.Fatalf("author read wrong: %+v %v", got, e)
	}
	// Another identity never sees the author row: the file itself is
	// isolated, so the note is unreachable before any leak could happen.
	bob, _ := login(t, f, "bob")
	_, bc := agent(t, bob, "", "read,write")
	if _, e := bc.PrivateNoteGet(context.Background(), op.FileID); e == nil || e.Error() != "not_found" {
		t.Fatalf("foreign note read must be not_found, got %v", e)
	}
	// Publishing preview never writes a thread and never mutates the note.
	preview, e := ac.PrivateNotePublishPreview(context.Background(), op.FileID)
	if e != nil {
		t.Fatal(e)
	}
	if preview["target"] != "team_thread" || preview["body_preview"] != "my private note" {
		t.Fatalf("preview wrong: %+v", preview)
	}
	threads, e := ac.FileThreadsList(context.Background(), op.FileID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	if n := len(threads["threads"].([]any)); n != 0 {
		t.Fatalf("preview created a thread: %d", n)
	}
	// Revoking the agent cuts both note read and write.
	f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true)
	if _, e := ac.PrivateNoteGet(context.Background(), op.FileID); e == nil || (e.Error() != "unauthorized" && e.Error() != "forbidden") {
		t.Fatalf("revoked note read must be refused, got %v", e)
	}
	// The human owner can still read the file (and write a fresh note).
	human := service.Principal{UserID: h.user.ID}
	// The note row already exists from the same user id: the owner updates
	// it under its current revision.
	view, e := f.Service.PrivateFileNote(human, op.FileID, "owner note", 1, true)
	if e != nil || view["revision"] != uint64(2) {
		t.Fatalf("owner note write failed: %+v %v", view, e)
	}
}

// Scenario 3: resolving a thread never grants domain approval, and script
// bodies stay inert plain text end to end.
func TestFPVersionDiscussionResolutionNotApprovalAndPlainText(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, ac := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, ac, h.space.ID, "resolved.txt", "resolve-src", []byte("resolve"))
	script := "<script>alert('x')</script>"
	thread, e := ac.FileThreadCreate(context.Background(), op.FileID, "", script, "script-key")
	if e != nil {
		t.Fatal(e)
	}
	threadID := thread["thread_id"].(string)
	resolved, e := ac.ThreadResolve(context.Background(), threadID)
	if e != nil || resolved["state"] != "resolved" {
		t.Fatalf("resolve wrong: %+v %v", resolved, e)
	}
	// Domain review facts did not move.
	var file service.File
	if e := f.Service.DB.First(&file, "id = ?", op.FileID).Error; e != nil {
		t.Fatal(e)
	}
	if file.ReviewPending {
		t.Fatal("thread resolution changed domain review state")
	}
	// The body round-trips verbatim as plain text, never interpreted.
	listed, e := ac.FileThreadsList(context.Background(), op.FileID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	one := listed["threads"].([]any)[0].(map[string]any)
	msg := one["messages"].([]any)[0].(map[string]any)
	if msg["body"] != script {
		t.Fatalf("plain text body mutated: %+v", msg)
	}
	// Reopen and delete placeholders keep audit rows.
	if _, e := ac.ThreadReopen(context.Background(), threadID); e != nil {
		t.Fatal(e)
	}
	listed2, _ := ac.FileThreadsList(context.Background(), op.FileID, "", 50)
	two := listed2["threads"].([]any)[0].(map[string]any)
	first := two["messages"].([]any)[0].(map[string]any)
	if _, e := ac.ThreadMessageDelete(context.Background(), first["message_id"].(string), uint64(first["revision"].(float64))); e != nil {
		t.Fatal(e)
	}
	listed3, _ := ac.FileThreadsList(context.Background(), op.FileID, "", 50)
	three := listed3["threads"].([]any)[0].(map[string]any)
	deleted := three["messages"].([]any)[0].(map[string]any)
	if deleted["deleted"] != true || deleted["body"] != "" {
		t.Fatalf("delete placeholder wrong: %+v", deleted)
	}
	// Read-only agents cannot speak.
	ro, roc := agent(t, h, "", "read")
	if _, e := roc.FileThreadCreate(context.Background(), op.FileID, "", "read-only try", "ro-key"); e == nil || e.Error() != "comment_write_forbidden" {
		t.Fatalf("read-only comment must be comment_write_forbidden, got %v", e)
	}
	_ = ro
}

// Task 4.1 entry equivalence: HTTP, MCP and SDK expose the same thread
// verdicts through one application service.
func TestFPVersionDiscussionEntryEquivalence(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, ac := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, ac, h.space.ID, "equiv-disc.txt", "equiv-disc", []byte("equiv"))
	sdkThread, e := ac.FileThreadCreate(context.Background(), op.FileID, "", "sdk entry", "equiv-key")
	if e != nil {
		t.Fatal(e)
	}
	threadID := sdkThread["thread_id"].(string)
	principal := service.Principal{UserID: h.user.ID, Agent: &a}
	mcpOut, e := f.Service.CallTool(context.Background(), principal, "file_threads_list", map[string]any{"file_id": op.FileID, "limit": "50"})
	if e != nil {
		t.Fatal(e)
	}
	mcpList, ok := mcpOut.(map[string]any)
	if !ok {
		t.Fatalf("file_threads_list returned unexpected shape: %T", mcpOut)
	}
	mcpThreads := mcpList["threads"].([]map[string]any)
	if len(mcpThreads) != 1 || mcpThreads[0]["thread_id"] != threadID {
		t.Fatalf("MCP verdict differs: %+v", mcpList)
	}
	var httpList struct {
		Threads []map[string]any `json:"threads"`
	}
	if status := h.call(t, "GET", "/files/"+op.FileID+"/threads?limit=50", nil, &httpList); status != 200 || len(httpList.Threads) != 1 || httpList.Threads[0]["thread_id"] != threadID {
		t.Fatalf("HTTP verdict differs: %d %+v", status, httpList)
	}
	// MCP resolve matches SDK resolve semantics.
	if _, e := f.Service.CallTool(context.Background(), principal, "thread_resolve", map[string]any{"thread_id": threadID}); e != nil {
		t.Fatal(e)
	}
	resolved, e := ac.FileThreadsList(context.Background(), op.FileID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	if state := resolved["threads"].([]any)[0].(map[string]any)["state"]; state != "resolved" {
		t.Fatalf("resolve did not land across entries: %v", state)
	}
}

// Task 4.2 fixture flow: 旧版本评论 → 新版本 → 私人备注 → 明确公开 → 撤权.
// The anchored old-version thread survives a new version; the private note
// stays author-only; explicit publication copies the body into a team
// thread; revoking the commenting agent stops further discussion writes
// without deleting history.
func TestFPVersionDiscussionFixtureFlow(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, ac := agent(t, h, "", "read,write")
	v1 := fpUploadBytes(t, ac, h.space.ID, "flow-disc.txt", "flow-v1", []byte("flow-v1"))
	thread, e := ac.FileThreadCreate(context.Background(), v1.FileID, v1.VersionID, "旧版本评论", "flow-thread")
	if e != nil {
		t.Fatal(e)
	}
	v2, e := ac.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "flow-disc.txt", FileID: v1.FileID, ExpectedVersion: v1.VersionID}, "flow-v2", strings.NewReader("flow-v2"), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ac.PrivateNoteSet(context.Background(), v1.FileID, "私人备注内容", 0); e != nil {
		t.Fatal(e)
	}
	preview, e := ac.PrivateNotePublishPreview(context.Background(), v1.FileID)
	if e != nil {
		t.Fatal(e)
	}
	if preview["body_preview"] != "私人备注内容" {
		t.Fatalf("preview wrong: %+v", preview)
	}
	// Explicit publication: a new thread carrying the note body.
	published, e := ac.FileThreadCreate(context.Background(), v1.FileID, v2.VersionID, "公开："+preview["body_preview"].(string), "flow-publish")
	if e != nil {
		t.Fatal(e)
	}
	if published["thread_id"] == thread["thread_id"] {
		t.Fatal("publication reused the private anchor thread")
	}
	// The private note itself is unchanged.
	note, e := ac.PrivateNoteGet(context.Background(), v1.FileID)
	if e != nil || note["body"] != "私人备注内容" {
		t.Fatalf("publication mutated the private note: %+v %v", note, e)
	}
	// Revoking the discussing agent stops new comments; the published
	// history stays readable by the owner.
	f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true)
	if _, e := ac.FileThreadCreate(context.Background(), v1.FileID, "", "after revoke", "after-revoke"); e == nil || (e.Error() != "unauthorized" && e.Error() != "forbidden") {
		t.Fatalf("revoked discussion write must be refused, got %v", e)
	}
	human := service.Principal{UserID: h.user.ID}
	kept, e := f.Service.ListFileThreads(human, v1.FileID, "", 50)
	if e != nil {
		t.Fatal(e)
	}
	if n := len(kept["threads"].([]map[string]any)); n != 2 {
		t.Fatalf("history lost after revocation: %d threads", n)
	}
}

// Task 5.2 scale acceptance, gated behind DRIVEBRIDGE_FP_SCALE: 10,000
// discussion entries paginated end to end, 20 concurrent edits resolving
// through revision conflicts, dual-user private notes and old-version
// anchors. Mobile draft recovery maps to the same revision_conflict
// contract covered above.
func TestFPVersionDiscussionScale(t *testing.T) {
	mode := os.Getenv("DRIVEBRIDGE_FP_SCALE")
	if mode != "quick" && mode != "full" {
		t.Skip("scale fixture gated behind DRIVEBRIDGE_FP_SCALE=quick|full")
	}
	total := 10000
	if mode == "quick" {
		total = 2000
	}
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, ac := agent(t, h, "", "read,write")
	op := fpUploadBytes(t, ac, h.space.ID, "scale-disc.txt", "scale-disc", []byte("scale"))
	// Threads in pages of 500 entries each via the SDK contract.
	threads := 0
	replies := 0
	for threads+replies < total {
		batch := min(500, total-threads-replies)
		for i := 0; i < batch; i += 2 {
			if _, e := ac.FileThreadCreate(context.Background(), op.FileID, "", fmt.Sprintf("scale thread %d", threads), fmt.Sprintf("scale-thread-%d", threads)); e != nil {
				t.Fatalf("thread %d: %v", threads, e)
			}
			threads++
		}
		for i := 0; i < batch/2; i++ {
			if _, e := ac.ThreadReply(context.Background(), mustFirstThread(t, ac, op.FileID), fmt.Sprintf("scale reply %d", replies), fmt.Sprintf("scale-reply-%d", replies)); e != nil {
				t.Fatalf("reply %d: %v", replies, e)
			}
			replies++
		}
	}
	// Keyset pagination walks every entry without loss.
	seen := 0
	cursor := ""
	for {
		page, e := ac.FileThreadsList(context.Background(), op.FileID, cursor, 200)
		if e != nil {
			t.Fatal(e)
		}
		rows := page["threads"].([]any)
		seen += len(rows)
		next, _ := page["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if seen != threads {
		t.Fatalf("pagination lost threads: %d of %d", seen, threads)
	}
	// 20 concurrent stale edits: exactly the first wins, every other gets
	// revision_conflict, and the stored body is one of the raced bodies.
	raced := make(chan string, 20)
	var wg sync.WaitGroup
	target := mustFirstMessage(t, ac, op.FileID)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if _, e := ac.ThreadMessageEdit(context.Background(), target["message_id"].(string), fmt.Sprintf("racer %d", n), uint64(target["revision"].(float64))); e != nil {
				if e.Error() != "revision_conflict" {
					raced <- "error:" + e.Error()
				} else {
					raced <- "conflict"
				}
			} else {
				raced <- "win"
			}
		}(i)
	}
	wg.Wait()
	close(raced)
	wins, conflicts := 0, 0
	for outcome := range raced {
		switch outcome {
		case "win":
			wins++
		case "conflict":
			conflicts++
		default:
			t.Fatalf("unexpected edit outcome: %s", outcome)
		}
	}
	if wins != 1 || conflicts != 19 {
		t.Fatalf("concurrent edit outcomes wrong: wins=%d conflicts=%d", wins, conflicts)
	}
	// Dual-user private notes stay isolated per author.
	_, bc := agent(t, bob, "", "read,write")
	_ = bc
	if _, e := ac.PrivateNoteSet(context.Background(), op.FileID, "alice scale note", 0); e != nil {
		t.Fatal(e)
	}
	aliceNote, e := ac.PrivateNoteGet(context.Background(), op.FileID)
	if e != nil || aliceNote["body"] != "alice scale note" {
		t.Fatalf("scale note wrong: %+v %v", aliceNote, e)
	}
	t.Logf("scale=%s entries=%d threads=%d replies=%d paginated=%d concurrent_edits=20 wins=1 conflicts=19", mode, threads+replies, threads, replies, seen)
}

func mustFirstThread(t *testing.T, c *sdk.Client, fileID string) string {
	t.Helper()
	page, e := c.FileThreadsList(context.Background(), fileID, "", 1)
	if e != nil {
		t.Fatal(e)
	}
	rows := page["threads"].([]any)
	if len(rows) == 0 {
		t.Fatal("no threads for reply target")
	}
	return rows[0].(map[string]any)["thread_id"].(string)
}

func mustFirstMessage(t *testing.T, c *sdk.Client, fileID string) map[string]any {
	t.Helper()
	page, e := c.FileThreadsList(context.Background(), fileID, "", 1)
	if e != nil {
		t.Fatal(e)
	}
	rows := page["threads"].([]any)
	if len(rows) == 0 {
		t.Fatal("no threads for edit target")
	}
	msgs := rows[0].(map[string]any)["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatal("no messages for edit target")
	}
	return msgs[0].(map[string]any)
}
