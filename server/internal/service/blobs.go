package service

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// blobScope is the dedupe dimension of a space: plaintext spaces share by
// digest alone within the data owner, encrypted spaces only within the space.
func blobScope(sp Space) string {
	if sp.Encrypted {
		return sp.ID
	}
	return ""
}

// blobObjectKey is deterministic: plaintext `<instance>/blobs/<sha256>`,
// encrypted `<instance>/blobs/<space>/<sha256>`. Concurrent completions of the
// same digest write the same bytes (or equivalent self-describing ciphertext)
// to one key, so no duplicate object is ever produced.
func (s *Service) blobObjectKey(scope, sha string) string {
	if scope == "" {
		return s.Instance + "/blobs/" + sha
	}
	return s.Instance + "/blobs/" + scope + "/" + sha
}

func blobID(owner, scope, sha string) string {
	return Hash(owner + "\x00" + scope + "\x00" + sha)
}

// findReusableBlob returns the caller-scoped blob for an instant upload hit,
// or nil. Another owner's digest is never a hit.
func (s *Service) findReusableBlob(owner, scope, sha string) *Blob {
	var b Blob
	if s.DB.First(&b, "id = ?", blobID(owner, scope, sha)).Error != nil {
		return nil
	}
	return &b
}

// claimBlob records one more reference inside the publish transaction. The
// first writer creates the row; later completions of the same digest only
// raise the count and point at the existing object key.
func claimBlob(tx *gorm.DB, b Blob) (Blob, error) {
	var existing Blob
	err := tx.First(&existing, "id = ?", b.ID).Error
	if err == nil {
		if e := tx.Model(&existing).Update("refs", existing.Refs+1).Error; e != nil {
			return existing, code("database_unavailable")
		}
		existing.Refs++
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return existing, code("database_unavailable")
	}
	b.Refs = 1
	b.CreatedAt = time.Now().UTC()
	if e := tx.Create(&b).Error; e != nil {
		return b, code("database_unavailable")
	}
	return b, nil
}

// releaseBlob lowers the reference count on purge. The object itself is never
// deleted here; blob gc is the only object-removal path.
func releaseBlob(tx *gorm.DB, id string) error {
	var b Blob
	if tx.First(&b, "id = ?", id).Error != nil {
		return nil
	}
	if b.Refs > 0 {
		return tx.Model(&b).Update("refs", b.Refs-1).Error
	}
	return nil
}

// BlobStat reports operator-visible metadata for one digest, scoped to this
// deployment. No credentials, signed URLs, or byte content are returned.
func (s *Service) BlobStat(sha string) ([]Blob, error) {
	if !validHash(sha) {
		return nil, code("invalid_upload")
	}
	var blobs []Blob
	if e := s.DB.Where("sha256 = ?", sha).Find(&blobs).Error; e != nil {
		return nil, code("database_unavailable")
	}
	return blobs, nil
}

// BlobGC collects reference-count-zero blobs. dryRun only reports; a confirmed
// run deletes the object and its row. Counts reaching zero never delete by
// themselves.
func (s *Service) BlobGC(ctx context.Context, dryRun bool) (map[string]any, error) {
	var blobs []Blob
	if e := s.DB.Where("refs <= ?", 0).Find(&blobs).Error; e != nil {
		return nil, code("database_unavailable")
	}
	var bytes int64
	removed := 0
	for _, b := range blobs {
		bytes += b.Size
		if dryRun {
			continue
		}
		st, e := s.storageFor(b.Owner, b.ConnectionID)
		if e != nil {
			return nil, e
		}
		if e = st.Delete(ctx, b.ObjectKey); e != nil {
			return nil, e
		}
		if e = s.DB.Delete(&b).Error; e != nil {
			return nil, code("database_unavailable")
		}
		removed++
	}
	return map[string]any{"dry_run": dryRun, "reclaimable_blobs": len(blobs), "reclaimable_bytes": bytes, "removed": removed}, nil
}

// BlobInfo is the HTTP read model for GET /api/v1/blobs/{sha256}: safe
// metadata only, and only when the caller can read at least one version that
// references the blob.
func (s *Service) BlobInfo(p Principal, sha string) (map[string]any, error) {
	if !validHash(sha) {
		return nil, code("not_found")
	}
	var versions []Version
	if e := s.DB.Where("blob_sha256 = ?", sha).Find(&versions).Error; e != nil || len(versions) == 0 {
		return nil, code("not_found")
	}
	authorized := map[string]bool{}
	for _, v := range versions {
		var f File
		if s.DB.First(&f, "id = ? AND trashed = ?", v.FileID, false).Error != nil {
			continue
		}
		// Legacy mode has no per-space read grants, so ownership is the bar;
		// governance-aware deployments also honor explicit read permission.
		if v.Owner == p.dataOwner() || s.allow(p, f.SpaceID, f.Path, "read") == nil && s.governanceActive() {
			authorized[blobID(v.Owner, blobScopeForVersion(s, v), sha)] = true
		}
	}
	if len(authorized) == 0 {
		return nil, code("not_found")
	}
	var blobs []Blob
	if e := s.DB.Where("sha256 = ?", sha).Find(&blobs).Error; e != nil {
		return nil, code("database_unavailable")
	}
	var refs, size int64
	for _, b := range blobs {
		if authorized[b.ID] {
			refs += b.Refs
			size = b.Size
		}
	}
	return map[string]any{"sha256": sha, "size_bytes": size, "ref_count": refs}, nil
}

func blobScopeForVersion(s *Service, v Version) string {
	var f File
	if s.DB.First(&f, "id = ?", v.FileID).Error != nil {
		return ""
	}
	var sp Space
	if s.DB.First(&sp, "id = ?", f.SpaceID).Error != nil {
		return ""
	}
	return blobScope(sp)
}
