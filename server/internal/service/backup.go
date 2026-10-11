package service

// Backup manifest and verification: every metadata backup carries an object
// inventory derived from the shipped database itself, and verification stays
// read-only so an unverified backup is never mistaken for a restorable one.

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
)

const manifestSchemaVersion = 1

type ManifestEntry struct {
	VersionID  string `json:"version_id"`
	ObjectKey  string `json:"object_key"`
	BlobSHA256 string `json:"blob_sha256,omitempty"`
	SHA256     string `json:"sha256_plaintext"`
	Size       int64  `json:"size_bytes"`
}
type BackupManifest struct {
	SchemaVersion int             `json:"schema_version"`
	InstanceID    string          `json:"instance_id"`
	CreatedAt     time.Time       `json:"created_at"`
	Counts        map[string]int  `json:"counts"`
	Entries       []ManifestEntry `json:"entries"`
}

func openBackupDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
}

// CheckpointMetadata flushes the write-ahead log into metadata.sqlite so the
// shipped file is self-contained and read-only consumers see every row.
func (s *Service) CheckpointMetadata() error {
	return s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
}

// WriteBackupManifest exports the version→object-key→plaintext-digest
// inventory from the backed-up metadata database, so manifest and shipped
// bytes cannot drift. Includes soft-deleted files' versions: restore must
// reproduce trash semantics.
func WriteBackupManifest(backupDir string) error {
	instance, e := os.ReadFile(filepath.Join(backupDir, "instance.id"))
	if e != nil {
		return code("instance_metadata_missing_restore_required")
	}
	db, e := openBackupDB(filepath.Join(backupDir, "metadata.sqlite"))
	if e != nil {
		return code("manifest_unavailable")
	}
	defer db.Close()
	manifest := BackupManifest{SchemaVersion: manifestSchemaVersion, InstanceID: string(instance), CreatedAt: time.Now().UTC(), Counts: map[string]int{}}
	// Older metadata lacks blob_sha256; fall back to the legacy shape so
	// pre-CAS backups still produce a manifest.
	rows, e := db.Query("SELECT id, object_key, sha256, size, blob_sha256 FROM versions ORDER BY created, id")
	withBlob := e == nil
	if !withBlob {
		rows, e = db.Query("SELECT id, object_key, sha256, size FROM versions ORDER BY created, id")
	}
	if e != nil {
		return code("manifest_unavailable")
	}
	defer rows.Close()
	for rows.Next() {
		var entry ManifestEntry
		if withBlob {
			var blob sql.NullString
			if e = rows.Scan(&entry.VersionID, &entry.ObjectKey, &entry.SHA256, &entry.Size, &blob); e != nil {
				return code("manifest_unavailable")
			}
			entry.BlobSHA256 = blob.String
		} else if e = rows.Scan(&entry.VersionID, &entry.ObjectKey, &entry.SHA256, &entry.Size); e != nil {
			return code("manifest_unavailable")
		}
		manifest.Entries = append(manifest.Entries, entry)
	}
	if e = rows.Err(); e != nil {
		return code("manifest_unavailable")
	}
	for _, table := range []string{"files", "spaces"} {
		var count int
		if e = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil {
			return code("manifest_unavailable")
		}
		manifest.Counts[table] = count
	}
	manifest.Counts["versions"] = len(manifest.Entries)
	// Pre-encryption backups lack the encrypted column; absence means zero.
	var encrypted int
	if e = db.QueryRow("SELECT COUNT(*) FROM spaces WHERE encrypted = 1").Scan(&encrypted); e != nil {
		encrypted = 0
	}
	manifest.Counts["encrypted_spaces"] = encrypted
	// Blob inventory cross-reference; pre-CAS backups have no blobs table.
	var blobs int
	if e = db.QueryRow("SELECT COUNT(*) FROM blobs").Scan(&blobs); e == nil {
		manifest.Counts["blobs"] = blobs
	}
	body, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return code("manifest_unavailable")
	}
	out, e := os.OpenFile(filepath.Join(backupDir, "manifest.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return code("manifest_unavailable")
	}
	_, writeErr := out.Write(append(body, '\n'))
	syncErr := out.Sync()
	closeErr := out.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return code("manifest_unavailable")
	}
	return nil
}

func ReadBackupManifest(dir string) (BackupManifest, error) {
	body, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return BackupManifest{}, code("manifest_missing")
	}
	var m BackupManifest
	if json.Unmarshal(body, &m) != nil || m.SchemaVersion != manifestSchemaVersion {
		return m, code("manifest_invalid")
	}
	return m, nil
}

// CrossCheckManifest re-queries the backup's own metadata database and
// reports every drift as a fixed failure code.
func CrossCheckManifest(dir string, m BackupManifest) []string {
	db, e := openBackupDB(filepath.Join(dir, "metadata.sqlite"))
	if e != nil {
		return []string{"manifest_mismatch:metadata_unreadable"}
	}
	defer db.Close()
	var failures []string
	index := map[string]ManifestEntry{}
	for _, entry := range m.Entries {
		index[entry.VersionID] = entry
	}
	rows, e := db.Query("SELECT id, object_key, sha256, size, blob_sha256 FROM versions")
	withBlob := e == nil
	if !withBlob {
		rows, e = db.Query("SELECT id, object_key, sha256, size FROM versions")
	}
	if e != nil {
		return []string{"manifest_mismatch:metadata_unreadable"}
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var id, key, digest string
		var size int64
		var blob sql.NullString
		if withBlob {
			if e = rows.Scan(&id, &key, &digest, &size, &blob); e != nil {
				return append(failures, "manifest_mismatch:metadata_unreadable")
			}
		} else if e = rows.Scan(&id, &key, &digest, &size); e != nil {
			return append(failures, "manifest_mismatch:metadata_unreadable")
		}
		entry, ok := index[id]
		if !ok {
			failures = append(failures, "manifest_mismatch:version_missing_in_manifest:"+id)
			continue
		}
		seen[id] = true
		if entry.ObjectKey != key {
			failures = append(failures, "manifest_mismatch:object_key:"+id)
		}
		if entry.SHA256 != digest {
			failures = append(failures, "manifest_mismatch:sha256:"+id)
		}
		if entry.Size != size {
			failures = append(failures, "manifest_mismatch:size:"+id)
		}
		if withBlob && entry.BlobSHA256 != blob.String {
			failures = append(failures, "manifest_mismatch:blob_sha256:"+id)
		}
	}
	for _, entry := range m.Entries {
		if !seen[entry.VersionID] {
			failures = append(failures, "manifest_mismatch:version_unknown:"+entry.VersionID)
		}
	}
	// Blob reference counts must match the shipped versions: a shared blob
	// whose count drifts would be garbage-collected while still referenced.
	blobRows, e := db.Query("SELECT id, sha256, owner, space_id, refs FROM blobs")
	if e == nil {
		defer blobRows.Close()
		for blobRows.Next() {
			var id, sha, owner, scope string
			var refs int64
			if blobRows.Scan(&id, &sha, &owner, &scope, &refs) != nil {
				return append(failures, "manifest_mismatch:metadata_unreadable")
			}
			var actual int64
			if e := db.QueryRow("SELECT COUNT(*) FROM versions v JOIN files f ON f.id = v.file_id JOIN spaces sp ON sp.id = f.space_id WHERE v.blob_sha256 = ? AND v.owner = ? AND (CASE WHEN sp.encrypted = 1 THEN sp.id ELSE '' END) = ?", sha, owner, scope).Scan(&actual); e != nil {
				return append(failures, "manifest_mismatch:metadata_unreadable")
			}
			if actual != refs {
				failures = append(failures, "manifest_mismatch:blob_refs:"+id)
			}
		}
	}
	return failures
}

// BackupVerifyResult is the three-stage report; fixed failure codes are
// manifest_mismatch, object_missing and sample_unverifiable.
type BackupVerifyResult struct {
	ManifestEntries int      `json:"manifest_entries"`
	DBVersions      int      `json:"db_versions"`
	Sampled         int      `json:"sampled"`
	SampleLimit     int      `json:"sample_limit"`
	EncryptedSpaces int      `json:"encrypted_spaces"`
	Failures        []string `json:"failures,omitempty"`
	Ready           bool     `json:"ready"`
}

// VerifyBackup validates an existing backup directory read-only: (1) manifest
// vs metadata cross-check, (2) sampled object existence through the platform
// connection credentials, (3) deployment consistency of the three pieces.
func (s *Service) VerifyBackup(ctx context.Context, dir string, sampleLimit int) (BackupVerifyResult, error) {
	result := BackupVerifyResult{SampleLimit: sampleLimit}
	if sampleLimit <= 0 {
		result.SampleLimit = 50
		sampleLimit = result.SampleLimit
	}
	m, e := ReadBackupManifest(dir)
	if e != nil {
		return result, e
	}
	result.ManifestEntries = len(m.Entries)
	result.EncryptedSpaces = m.Counts["encrypted_spaces"]
	failures := CrossCheckManifest(dir, m)
	db, e := openBackupDB(filepath.Join(dir, "metadata.sqlite"))
	if e != nil {
		return result, code("manifest_unavailable")
	}
	defer db.Close()
	var dbVersions int
	if e = db.QueryRow("SELECT COUNT(*) FROM versions").Scan(&dbVersions); e != nil {
		return result, code("manifest_unavailable")
	}
	result.DBVersions = dbVersions
	if m.InstanceID != s.Instance {
		failures = append(failures, "manifest_mismatch:instance_id")
	}
	entries := m.Entries
	if len(entries) > 0 {
		step := len(entries) / min(sampleLimit, len(entries))
		if step < 1 {
			step = 1
		}
		for i := 0; i < len(entries); i += step {
			entry := entries[i]
			var owner, kind string
			var secret []byte
			row := db.QueryRow("SELECT c.owner, c.kind, c.secret FROM versions v JOIN connections c ON c.id = v.connection_id WHERE v.id = ?", entry.VersionID)
			if e := row.Scan(&owner, &kind, &secret); e != nil || kind != "s3" {
				failures = append(failures, "sample_unverifiable:"+entry.VersionID)
				result.Sampled++
				continue
			}
			var cfg S3Config
			if s.unseal(owner, secret, &cfg) != nil {
				failures = append(failures, "sample_unverifiable:"+entry.VersionID)
				result.Sampled++
				continue
			}
			store, e := NewS3(cfg, s.Config.Development)
			if e != nil {
				failures = append(failures, "sample_unverifiable:"+entry.VersionID)
				result.Sampled++
				continue
			}
			if e := store.Stat(ctx, entry.ObjectKey); e != nil {
				failures = append(failures, "object_missing:"+entry.VersionID)
			}
			result.Sampled++
		}
	}
	sort.Strings(failures)
	result.Failures = failures
	result.Ready = len(failures) == 0
	return result, nil
}

// SealBundle packs named files into an AES-GCM sealed tarball under the
// purpose-separated instance-key derivation.
func (s *Service) SealBundle(files map[string][]byte) ([]byte, error) {
	aead, e := s.backupAEAD()
	if e != nil {
		return nil, e
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		if e = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(files[name]))}); e != nil {
			return nil, code("encryption_unavailable")
		}
		if _, e = tw.Write(files[name]); e != nil {
			return nil, code("encryption_unavailable")
		}
	}
	if e = tw.Close(); e != nil {
		return nil, code("encryption_unavailable")
	}
	nonce := make([]byte, 12)
	rand.Read(nonce)
	out := append([]byte("DBB1"), nonce...)
	return append(out, aead.Seal(nil, nonce, buf.Bytes(), []byte("backup_bundle"))...), nil
}

// OpenBundle decrypts and unpacks a sealed backup tarball.
func (s *Service) OpenBundle(blob []byte) (map[string][]byte, error) {
	if len(blob) < 4+12+16 || string(blob[:4]) != "DBB1" {
		return nil, code("content_mismatch")
	}
	aead, e := s.backupAEAD()
	if e != nil {
		return nil, e
	}
	plain, e := aead.Open(nil, blob[4:16], blob[16:], []byte("backup_bundle"))
	if e != nil {
		return nil, code("content_mismatch")
	}
	tr := tar.NewReader(bytes.NewReader(plain))
	files := map[string][]byte{}
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil || hdr.Size > 512<<20 || (hdr.Mode != 0600 && hdr.Mode != 0) {
			return nil, code("content_mismatch")
		}
		if hdr.Name == "" || strings.ContainsAny(hdr.Name, "/\\\x00") || strings.HasPrefix(hdr.Name, ".") {
			return nil, code("content_mismatch")
		}
		body, e := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
		if e != nil || int64(len(body)) != hdr.Size {
			return nil, code("content_mismatch")
		}
		files[hdr.Name] = body
	}
	if len(files) == 0 {
		return nil, code("content_mismatch")
	}
	return files, nil
}
