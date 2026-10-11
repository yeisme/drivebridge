package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"gorm.io/gorm"
)

type workspaceObservedBody struct {
	io.ReadCloser
	closed bool
}

func (b *workspaceObservedBody) Close() error { b.closed = true; return b.ReadCloser.Close() }

type workspaceAfterGetStore struct {
	Storage
	after func()
	body  *workspaceObservedBody
}

func (s *workspaceAfterGetStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := s.Storage.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	s.body = &workspaceObservedBody{ReadCloser: body}
	s.after()
	return s.body, nil
}

func TestWorkspaceDownloadSpaceFailureClosesBody(t *testing.T) {
	s, store, p := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	uploaded := uploadBytes(t, s, p, sp.ID, "file.txt", []byte("fixture"))
	const callback = "fixture_download_space_failure"
	adapter := &workspaceAfterGetStore{Storage: store, after: func() {
		if err := s.DB.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*Space); ok {
				tx.AddError(errors.New("fixture failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
	}}
	s.storage = func(S3Config, bool) (Storage, error) { return adapter, nil }
	_, _, body, err := s.Download(t.Context(), p, uploaded.FileID, uploaded.VersionID)
	if removeErr := s.DB.Callback().Query().Remove(callback); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err == nil || err.Error() != "database_unavailable" || body != nil {
		t.Fatal("space failure returned unchecked bytes")
	}
	if adapter.body == nil || !adapter.body.closed {
		t.Fatal("failure leaked storage body")
	}
}
