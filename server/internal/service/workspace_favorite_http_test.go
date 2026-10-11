package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceFavoriteHTTPIdentityAndRollback(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	file := orgUpload(t, mustSDK(t, h), h.space.ID, "favorite-http/note.md", "# Note")
	if status := h.call(t, "POST", "/directories", map[string]string{"space_id": h.space.ID, "path": "favorite-empty"}, nil); status != 200 {
		t.Fatal(status)
	}
	collection := service.FileCollection{ID: service.ID(), Owner: p.UserID, SpaceID: h.space.ID, Name: "Favorite collection", Kind: "manual", Scope: "private", Revision: 1}
	if err := f.Service.DB.Create(&collection).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Create(&service.FileLabelAssignment{ID: service.ID(), Owner: p.UserID, FileID: file.FileID, Namespace: "manual", Label: "reference", AuthorKind: "human"}).Error; err != nil {
		t.Fatal(err)
	}
	base := "/favorites"
	prefix := "drivebridge://" + f.Service.Instance
	var first contract.FavoriteReceipt
	for _, target := range []struct{ kind, ref string }{
		{"file", prefix + "/file/" + file.FileID},
		{"directory", prefix + "/connection/" + h.space.ConnectionID + "/root/" + h.space.ID + "/directory/favorite-empty"},
		{"collection", prefix + "/collection/" + collection.ID},
		{"tag", prefix + "/tag/manual/reference"},
	} {
		q := contract.FavoriteCreate{SpaceID: h.space.ID, TargetKind: target.kind, TargetRef: target.ref}
		raw, _ := json.Marshal(q)
		req, _ := http.NewRequest("POST", h.base+"/api/v1"+base, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.Header.Set("Origin", h.base)
		req.Header.Set("Idempotency-Key", "http-favorite-"+target.kind)
		res, err := h.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var env contract.DataEnvelope[contract.FavoriteReceipt]
		decodeErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if res.StatusCode != 200 || decodeErr != nil || env.Data.ResultRevision != 1 {
			t.Fatal(target.kind, res.StatusCode, decodeErr)
		}
		if target.kind == "file" {
			first = env.Data
		}
	}
	var list contract.FavoriteList
	if status := h.call(t, "GET", base+"?space_id="+h.space.ID, nil, &list); status != 200 || len(list.Items) != 4 || list.Total != nil || list.TotalKind != "unknown" || list.ObservedAt.IsZero() {
		t.Fatal(status, list)
	}
	var receipt contract.FavoriteReceipt
	if status := h.call(t, "POST", base+"/lookup", contract.FavoriteLookup{SpaceID: h.space.ID, Action: "create", IdempotencyKey: "http-favorite-file"}, &receipt); status != 200 || receipt != first {
		t.Fatal("HTTP recovery", status)
	}
	for _, bad := range []string{`{}`, `{"target_kind":"file","target_ref":"x","expected_revision":null}`, `{"target_kind":"file","target_ref":"x","expected_revision":0,"owner":"spoof"}`} {
		if status, _ := fpRawCall(t, h, "POST", base, []byte(bad)); status != 400 {
			t.Fatal("open mutation DTO", status)
		}
	}
	if status := h.call(t, "GET", base+"?unknown=1", nil, nil); status != 400 {
		t.Fatal(status)
	}
	// Binding mutation and its receipt must roll back together.
	f.Service.DB.Callback().Create().Before("gorm:create").Register("favorite_receipt_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" {
			tx.AddError(errors.New("synthetic receipt failure"))
		}
	})
	_, failure := f.Service.DeleteWorkspaceFavorite(t.Context(), p, first.FavoriteID, contract.FavoriteDelete{ExpectedRevision: 1}, "rollback-delete")
	f.Service.DB.Callback().Create().Remove("favorite_receipt_failure")
	if failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal(failure)
	}
	var row service.WorkspaceFavorite
	if err := f.Service.DB.First(&row, "id = ?", first.FavoriteID).Error; err != nil || row.Revision != 1 || row.Deleted {
		t.Fatal("partial binding mutation", row, err)
	}
	raw, _ := json.Marshal(contract.FavoriteDelete{ExpectedRevision: 1})
	req, _ := http.NewRequest("DELETE", h.base+"/api/v1"+base+"/"+first.FavoriteID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", "http-favorite-delete")
	res, err := h.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var deleted contract.DataEnvelope[contract.FavoriteReceipt]
	decodeErr := json.NewDecoder(res.Body).Decode(&deleted)
	res.Body.Close()
	if res.StatusCode != 200 || decodeErr != nil || !deleted.Data.Deleted || deleted.Data.ResultRevision != 2 {
		t.Fatal("HTTP delete failed", res.StatusCode, decodeErr)
	}
	if status := h.call(t, "POST", base+"/lookup", contract.FavoriteLookup{SpaceID: h.space.ID, Action: "delete", IdempotencyKey: "http-favorite-delete"}, &receipt); status != 200 || receipt != deleted.Data {
		t.Fatal("HTTP delete recovery", status)
	}
}
