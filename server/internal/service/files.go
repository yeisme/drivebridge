package service

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/sdk/insights"
	"gorm.io/gorm"
)

const PartSize int64 = 8 << 20

type UploadRequest struct {
	ReviewPending   bool   `json:"review_pending,omitempty"`
	Caller          string `json:"caller,omitempty"`
	TaskRef         string `json:"task_ref,omitempty"`
	SpaceID         string `json:"space_id"`
	Path            string `json:"path"`
	Size            int64  `json:"size_bytes"`
	SHA256          string `json:"sha256"`
	FileID          string `json:"file_id,omitempty"`
	ExpectedVersion string `json:"expected_version,omitempty"`
	// EntitlementID references the Aigora service entitlement admitted for
	// hosted (unified identity) uploads; legacy principals ignore it.
	EntitlementID string `json:"entitlement_id,omitempty"`
	// BatchID/ItemKey bind this upload to one scheduled upload-batch item.
	// Both are optional so the legacy single-file contract is unchanged, and
	// both join the idempotency fingerprint because they are business inputs.
	BatchID string `json:"batch_id,omitempty"`
	ItemKey string `json:"item_key,omitempty"`
}

var provenanceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,159}$`)

func validProvenance(caller, task string) bool {
	for _, v := range []string{caller, task} {
		if v != "" && !provenanceID.MatchString(v) {
			return false
		}
	}
	return true
}

func validHash(h string) bool {
	b, e := hex.DecodeString(h)
	return e == nil && len(b) == 32 && strings.ToLower(h) == h
}
func (s *Service) CreateUpload(p Principal, q UploadRequest, key string) (Operation, error) {
	return s.createUpload(p, q, key, nil)
}

func (s *Service) createUpload(p Principal, q UploadRequest, key string, origin *ingestOrigin) (Operation, error) {
	if !validProvenance(q.Caller, q.TaskRef) {
		return Operation{}, code("invalid_provenance")
	}
	if key == "" || len(key) > 200 || q.Size < 0 || q.Size > s.Config.MaxBytes || !validHash(q.SHA256) {
		return Operation{}, code("invalid_upload")
	}
	if _, e := cleanPath(q.Path); e != nil {
		return Operation{}, e
	}
	if e := s.allow(p, q.SpaceID, q.Path, "write"); e != nil {
		return Operation{}, e
	}
	if q.FileID != "" && s.governanceActive() {
		if e := s.allow(p, q.SpaceID, q.Path, "overwrite"); e != nil {
			return Operation{}, e
		}
	}
	body, _ := json.Marshal(q)
	fingerprint := Hash(string(body))
	if origin != nil {
		body, _ = json.Marshal(origin)
		fingerprint = Hash(origin.Kind + ":" + string(body))
	}
	// Hosted (unified identity) uploads are admitted at this business
	// operation boundary: usage is reserved with the authority before any
	// local state changes, and the reservation is journalled with the
	// operation. Legacy principals are never billed.
	var admission *ServiceAdmission
	operationID, versionID := ID(), ID()
	if p.ServiceIdentity != nil {
		var replay Operation
		if err := s.DB.First(&replay, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			if replay.Fingerprint != fingerprint {
				return Operation{}, code("idempotency_conflict")
			}
			return replay, nil
		}
		reserved, admissionErr := s.reserveServiceAdmission(p, operationID, q.Size, q.EntitlementID)
		if admissionErr != nil {
			return Operation{}, admissionErr
		}
		admission = reserved
	}
	// Instant upload: an existing same-owner blob in the caller's write scope
	// completes with zero bytes transferred. The object is probed first —
	// metadata without a readable object is drift (blob_unavailable), never a
	// hit. Authorization, quota and idempotency checks below still apply.
	// Version replace and archive flows are excluded; Finder copy may reuse.
	var reuse *Blob
	if (origin == nil || origin.Kind == "copy") && q.FileID == "" && p.ServiceIdentity == nil {
		var sp Space
		if s.DB.First(&sp, "id = ? AND owner = ?", q.SpaceID, p.dataOwner()).Error == nil {
			if b := s.findReusableBlob(p.dataOwner(), blobScope(sp), q.SHA256); b != nil && b.Size == q.Size {
				st, stErr := s.storageForSpace(p.dataOwner(), q.SpaceID, b.ConnectionID)
				if stErr != nil {
					return Operation{}, stErr
				}
				if stErr = st.Stat(context.Background(), b.ObjectKey); stErr != nil {
					return Operation{}, code("blob_unavailable")
				}
				reuse = b
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var op Operation
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := teamWritable(tx, q.SpaceID); err != nil {
			return err
		}
		var old Operation
		if tx.First(&old, "owner = ? AND key = ?", p.dataOwner(), key).Error == nil {
			if old.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			op = old
			return nil
		}
		var boundBatch *UploadBatch
		if q.BatchID != "" {
			bound, err := s.bindBatchItem(tx, p, &q)
			if err != nil {
				return err
			}
			boundBatch = &bound
		}
		var sp Space
		if tx.First(&sp, "id = ? AND owner = ?", q.SpaceID, p.dataOwner()).Error != nil {
			return code("not_found")
		}
		if sp.ConnectionID == "" {
			return code("storage_disconnected")
		}
		var conn Connection
		if tx.First(&conn, "id = ? AND disabled = ?", sp.ConnectionID, false).Error != nil {
			return code("storage_disconnected")
		}
		if q.Size > sp.MaxBytes-sp.Used-sp.Reserved {
			return code("quota_exceeded")
		}
		var nonce []byte
		if sp.Encrypted {
			nonce = make([]byte, 12)
			rand.Read(nonce)
		}
		if origin == nil || !origin.AutoRename {
			if err := pathAvailable(tx, p.dataOwner(), q.SpaceID, q.Path, q.FileID); err != nil {
				// keep_both batches rename instead of failing: the client chose
				// the policy explicitly at batch creation, so a taken name
				// resolves to a sibling name instead of name_conflict.
				if boundBatch == nil || boundBatch.ConflictPolicy != "keep_both" || q.FileID != "" {
					return err
				}
				renamed, rerr := archivePath(tx, q.SpaceID, q.Path)
				if rerr != nil {
					return rerr
				}
				if aerr := s.allowOn(tx, p, q.SpaceID, renamed, "write"); aerr != nil {
					return aerr
				}
				q.Path = renamed
			}
		}
		var f File
		if q.FileID != "" {
			if tx.First(&f, "id = ? AND owner = ? AND space_id = ? AND path = ? AND trashed = ?", q.FileID, p.dataOwner(), q.SpaceID, q.Path, false).Error != nil {
				return code("not_found")
			}
			if q.ExpectedVersion == "" || f.VersionID != q.ExpectedVersion {
				return code("version_conflict")
			}
		} else {
			if origin != nil && origin.AutoRename {
				var err error
				q.Path, err = archivePath(tx, q.SpaceID, q.Path)
				if err != nil {
					return err
				}
				if err = s.allowOn(tx, p, q.SpaceID, q.Path, "write"); err != nil {
					return err
				}
			}
			var count int64
			tx.Model(&File{}).Where("space_id = ? AND path = ?", q.SpaceID, q.Path).Count(&count)
			if count != 0 {
				return code("name_conflict")
			}
		}
		if reuse != nil {
			var blob Blob
			if tx.First(&blob, "id = ?", reuse.ID).Error != nil {
				return code("blob_unavailable")
			}
			if e := tx.Model(&blob).Update("refs", blob.Refs+1).Error; e != nil {
				return code("database_unavailable")
			}
			f = File{ReviewPending: q.ReviewPending, ID: ID(), Owner: p.dataOwner(), SpaceID: q.SpaceID, Path: q.Path}
			if e := tx.Create(&f).Error; e != nil {
				return code("name_conflict")
			}
			v := Version{Caller: q.Caller, TaskRef: q.TaskRef, ID: versionID, Owner: p.dataOwner(), FileID: f.ID, ConnectionID: blob.ConnectionID, ObjectKey: blob.ObjectKey, BlobSHA256: q.SHA256, SHA256: q.SHA256, Size: q.Size, MIME: blob.MIME, Created: time.Now().UTC()}
			if e := tx.Create(&v).Error; e != nil {
				return code("database_unavailable")
			}
			f.ReviewPending = q.ReviewPending
			f.Caller, f.TaskRef = q.Caller, q.TaskRef
			f.VersionID, f.Size, f.MIME, f.Updated = v.ID, v.Size, v.MIME, v.Created
			if e := tx.Save(&f).Error; e != nil {
				return code("database_unavailable")
			}
			agentID := ""
			if p.Agent != nil {
				agentID = p.Agent.ID
			}
			op = Operation{ActorID: p.UserID, ReviewPending: q.ReviewPending, Caller: q.Caller, TaskRef: q.TaskRef, ID: operationID, Owner: p.dataOwner(), AgentID: agentID, Key: key, Fingerprint: fingerprint, Kind: "upload", SpaceID: q.SpaceID, Path: q.Path, Size: q.Size, SHA256: q.SHA256, FileID: f.ID, VersionID: versionID, ConnectionID: blob.ConnectionID, Status: "completed", Transferred: 0, BlobReused: true, MIME: blob.MIME, BatchID: q.BatchID, ItemKey: q.ItemKey, Updated: v.Created}
			if origin != nil {
				op.Kind = origin.Kind
				op.ArchiveFileID, op.ArchiveVersionID, op.ArchiveHeadVersion = origin.Source.SourceFileID, origin.Source.SourceVersionID, origin.Source.ExpectedSourceVersion
			}
			if e := tx.Create(&op).Error; e != nil {
				return code("database_unavailable")
			}
			if q.BatchID != "" {
				// An instant reuse completes the item inside this transaction; no
				// session is needed because no client part will ever arrive.
				if err := scheduleBatchItem(tx, p.dataOwner(), q.BatchID, q.ItemKey, op.ID); err != nil {
					return code("database_unavailable")
				}
			}
			if admission != nil {
				if e := tx.Create(admission).Error; e != nil {
					return code("database_unavailable")
				}
				if e := s.event(tx, Principal{UserID: admission.UserID, TenantID: admission.TenantID}, Space{}, "admission.reserved", admission.OperationRef, op.ID, "success", admission.EntitlementID); e != nil {
					return e
				}
			}
			// Quota was validated above; an instant hit settles immediately, so
			// only the logical usage moves and no reservation remains.
			if e := tx.Model(&sp).Update("used", sp.Used+q.Size).Error; e != nil {
				return e
			}
			// The blob-reuse instant completion is a committed file too:
			// its catalogue event joins this transaction so a deduplicated
			// upload is never missing from the outbox.
			return s.recordFileEvent(tx, p.dataOwner(), f.ID, fileRevision(f.Updated), false, map[string]any{"path": f.Path, "size_bytes": f.Size, "mime_type": f.MIME, "trashed": false})
		}
		agent := ""
		if p.Agent != nil {
			agent = p.Agent.ID
		}
		op = Operation{ActorID: p.UserID, ReviewPending: q.ReviewPending, Caller: q.Caller, TaskRef: q.TaskRef, ID: operationID, Owner: p.dataOwner(), AgentID: agent, Key: key, Fingerprint: fingerprint, Kind: "upload", SpaceID: q.SpaceID, Path: q.Path, Size: q.Size, SHA256: q.SHA256, FileID: q.FileID, ExpectedVersion: q.ExpectedVersion, VersionID: versionID, ConnectionID: sp.ConnectionID, Nonce: nonce, Status: "awaiting_upload", BatchID: q.BatchID, ItemKey: q.ItemKey, Updated: time.Now().UTC()}
		if origin != nil {
			op.Kind = origin.Kind
			op.RenameOnConflict = origin.AutoRename
			op.ArchiveFileID, op.ArchiveVersionID, op.ArchiveHeadVersion = origin.Source.SourceFileID, origin.Source.SourceVersionID, origin.Source.ExpectedSourceVersion
		}
		if e := tx.Create(&op).Error; e != nil {
			return code("database_unavailable")
		}
		if err := s.bindWorkspaceArchiveOn(tx, p, op, origin); err != nil {
			return err
		}
		if q.BatchID != "" {
			if err := scheduleBatchItem(tx, p.dataOwner(), q.BatchID, q.ItemKey, op.ID); err != nil {
				return code("database_unavailable")
			}
			if err := s.ensureUploadSession(tx, op); err != nil {
				return code("database_unavailable")
			}
		}
		if admission != nil {
			if e := tx.Create(admission).Error; e != nil {
				return code("database_unavailable")
			}
			if e := s.event(tx, Principal{UserID: admission.UserID, TenantID: admission.TenantID}, Space{}, "admission.reserved", admission.OperationRef, op.ID, "success", admission.EntitlementID); e != nil {
				return e
			}
		}
		return tx.Model(&sp).Update("reserved", sp.Reserved+q.Size).Error
	})
	if e == nil && reuse != nil && admission != nil {
		// An instant hit delivered zero new bytes; the reservation settles now.
		_ = s.reportServiceAdmission(op, "delivered", op.Size)
	}
	if e == nil && op.BatchID != "" && op.Status == "completed" {
		// Instant-reuse items complete inside the creation transaction; the
		// batch counters catch up here, outside the business rows.
		s.finishBatchItem(op, "completed", "")
	}
	if e != nil && admission != nil {
		// The transaction rolled back the local journal row, so reconcile
		// would never find this reservation; settle it from memory or the
		// authority holds the units forever.
		_ = s.reportAdmissionRef(*admission, "failed", 0)
	}
	return op, e
}
func (s *Service) PutPart(p Principal, id string, index int, reader io.Reader) (Operation, error) {
	return s.putPart(p, id, index, reader, false)
}
func (s *Service) opLock(id string) func() {
	h := sha256.Sum256([]byte(id))
	m := &s.opLocks[int(h[0])%len(s.opLocks)]
	m.Lock()
	return m.Unlock
}
func (s *Service) putPart(p Principal, id string, index int, reader io.Reader, archive bool) (Operation, error) {
	return s.putPartAuthorized(p, id, index, reader, archive, false)
}

func (s *Service) putPartAuthorized(p Principal, id string, index int, reader io.Reader, archive, workspace bool) (Operation, error) {
	unlock := s.opLock(id)
	defer unlock()
	load := func(db *gorm.DB) (Operation, error) {
		if workspace {
			op, _, err := s.workspaceOperationOn(db, p, id, time.Now().UTC())
			return op, err
		}
		return s.operation(p, id, "write")
	}
	op, e := load(s.DB)
	if e != nil {
		return op, e
	}
	if op.Kind != "upload" && op.Kind != "import" && !(archive && (op.Kind == "archive" || op.Kind == "copy")) {
		return op, code("invalid_operation")
	}
	if op.Status == "cancelled" || op.Status == "completed" || op.Status == "verifying" {
		return op, code("operation_closed")
	}
	if op.BatchID != "" && op.ItemKey != "" {
		var session UploadSession
		if s.DB.First(&session, "operation_id = ?", op.ID).Error == nil && uploadSessionExpired(session) {
			// The backend transfer window lapsed: new parts would land on a
			// dead backend multipart session. The original actor's explicit
			// resume rebuilds the session under this same operation.
			return op, code("transfer_session_expired")
		}
	}
	meter := &insights.ObservedReader{Reader: reader}
	reader = meter
	defer func() {
		direction := "client_to_server"
		purpose := "transfer"
		if op.Kind == "import" {
			direction = "storage_to_server"
			purpose = "import"
		}
		s.observed(p, op, direction, purpose, meter.Bytes)
	}()
	count := (op.Size + PartSize - 1) / PartSize
	if index < 0 || int64(index) >= count {
		return op, code("invalid_part")
	}
	size := PartSize
	if int64(index) == count-1 {
		size = op.Size - int64(index)*PartSize
	}
	dir := filepath.Join(s.Config.State, "uploads", id)
	if e = privateDir(dir); e != nil {
		return op, e
	}
	var sp Space
	encrypted := false
	if s.DB.First(&sp, "id = ? AND owner = ?", op.SpaceID, p.dataOwner()).Error == nil && sp.Encrypted {
		encrypted = true
	}
	f, e := os.CreateTemp(dir, "part-*")
	if e != nil {
		return op, code("disk_write_failed")
	}
	temp := f.Name()
	defer os.Remove(temp)
	hash := sha256.New()
	staged := size
	if encrypted {
		// Staged bytes stay ciphertext: recovery replays them verbatim and no
		// plaintext intermediate is written to local disk.
		var aead cipher.AEAD
		aead, e = s.spaceContentAEAD(sp)
		if e == nil && len(op.Nonce) != 12 {
			e = code("encryption_unavailable")
		}
		var sealer *contentSealer
		if e == nil {
			count := (op.Size + PartSize - 1) / PartSize
			startChunk := int64(index) * PartSize / contentChunk
			finalChunk := int64(-1)
			if int64(index) == count-1 {
				finalChunk = (op.Size - 1) / contentChunk
			}
			sealer = newContentSealer(aead, op.Nonce, startChunk, finalChunk, op.Size, f)
			var n int64
			n, e = io.Copy(io.MultiWriter(hash, sealer), io.LimitReader(reader, size+1))
			if e == nil && n != size {
				e = code("incomplete_part")
			}
			if e == nil {
				e = sealer.Close()
			}
			if e == nil && sealer.written != contentFrameSize(size) {
				e = code("incomplete_part")
			}
			staged = sealer.written
		}
		syncErr := f.Sync()
		f.Close()
		if e != nil || syncErr != nil {
			return op, code("incomplete_part")
		}
	} else {
		var n int64
		n, e = io.Copy(io.MultiWriter(f, hash), io.LimitReader(reader, size+1))
		syncErr := f.Sync()
		f.Close()
		if e != nil || syncErr != nil || n != size {
			return op, code("incomplete_part")
		}
	}
	if workspace {
		if _, err := load(s.DB); err != nil {
			return op, err
		}
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	var existing Part
	if s.DB.First(&existing, "operation_id = ? AND `index` = ?", id, index).Error == nil {
		if existing.SHA256 != digest || existing.Size != staged {
			return op, code("part_conflict")
		}
		return op, nil
	}
	target := filepath.Join(dir, fmt.Sprintf("%08d", index))
	if e = os.Rename(temp, target); e != nil {
		return op, code("disk_write_failed")
	}
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if workspace {
			current, err := load(tx)
			if err != nil {
				return err
			}
			if current.Status != "awaiting_upload" && current.Status != "transferring" {
				return code("operation_closed")
			}
		}
		if e := tx.Create(&Part{OperationID: id, Index: index, Size: staged, SHA256: digest}).Error; e != nil {
			return e
		}
		return tx.Model(&Operation{}).Where("id = ?", id).Updates(map[string]any{"transferred": op.Transferred + size, "status": "transferring", "error": "", "updated": time.Now().UTC()}).Error
	})
	if e != nil {
		if workspace {
			// No Part row committed; do not leave accepted-looking staged bytes.
			_ = os.Remove(target)
		}
		return op, code("database_unavailable")
	}
	return load(s.DB)
}
func (s *Service) Complete(ctx context.Context, p Principal, id string) (Operation, error) {
	return s.completeAuthorized(ctx, p, id, false)
}

func (s *Service) completeAuthorized(ctx context.Context, p Principal, id string, workspace bool) (result Operation, resultErr error) {
	unlock := s.opLock(id)
	defer unlock()
	var op Operation
	var e error
	if workspace {
		op, _, e = s.workspaceOperationOn(s.DB, p, id, time.Now().UTC())
	} else {
		op, e = s.operation(p, id, "write")
	}
	if e != nil {
		return op, e
	}
	if op.Kind != "upload" && op.Kind != "import" && op.Kind != "archive" && op.Kind != "copy" {
		return op, code("invalid_operation")
	}
	ctx, finish := s.operationContext(ctx, id)
	defer finish()
	if op.Status == "completed" {
		return op, nil
	}
	if op.Kind == "archive" || op.Kind == "copy" {
		if e := s.validateArchiveSource(p, op); e != nil {
			return op, e
		}
	}
	if op.Status == "cancelled" {
		return op, code("operation_closed")
	}
	if op.Error == "original_source_required" {
		// Metadata-only restore lost the staged bytes: only the original
		// actor re-selecting the source recovers this operation; reporting
		// missing_parts would invite blind part resends against nothing.
		return op, code("source_reselection_required")
	}
	defer func() {
		if s.Insights != nil && resultErr != nil {
			_ = s.Insights.Record(insights.Fact{Dimensions: s.dimensions(p, op), Metric: "failed_attempt", Purpose: "completion"})
		}
	}()
	if op.ExpectedVersion != "" {
		if e := s.requireAudit(p, "files.version_replace", op.FileID); e != nil {
			return op, e
		}
	}
	storage, e := s.storageForSpace(p.dataOwner(), op.SpaceID, op.ConnectionID)
	if e != nil {
		return op, e
	}
	var sp Space
	var aead cipher.AEAD
	encrypted := false
	if s.DB.First(&sp, "id = ? AND owner = ?", op.SpaceID, p.dataOwner()).Error == nil && sp.Encrypted {
		if len(op.Nonce) != 12 || len(sp.WrappedDEK) == 0 {
			return op, code("encryption_unavailable")
		}
		if aead, e = s.spaceContentAEAD(sp); e != nil {
			return op, e
		}
		encrypted = true
	}
	if op.BatchID != "" && !encrypted {
		// Streaming backend reassembly for batch transfers: verified client
		// parts flow through bounded buffers straight into the target object
		// and the read-back verification streams the stored bytes, so a large
		// file never exists as a second assembled local copy. Encrypted
		// spaces keep the self-describing assembled layout below.
		mime, _, se := s.streamUploadToObject(ctx, p, op, sp, storage)
		if se != nil {
			return op, se
		}
		return s.publishUpload(p, op, encrypted, mime, filepath.Join(s.Config.State, "uploads", id))
	}
	dir := filepath.Join(s.Config.State, "uploads", id)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return op, code("disk_write_failed")
	}
	f, e := os.CreateTemp(dir, "assembled-*")
	if e != nil {
		return op, code("disk_write_failed")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	mime := ""
	objectSize := op.Size
	if encrypted {
		// The object header carries the wrap version and the operation nonce;
		// staged parts already hold the framed ciphertext.
		if _, e = f.Write([]byte(contentMagic)); e != nil {
			return op, code("disk_write_failed")
		}
		if _, e = f.Write([]byte{sp.WrappedDEK[0]}); e != nil {
			return op, code("disk_write_failed")
		}
		if _, e = f.Write(op.Nonce); e != nil {
			return op, code("disk_write_failed")
		}
		if op.Size == 0 {
			sealer := newContentSealer(aead, op.Nonce, 0, 0, 0, f)
			if e = sealer.Close(); e != nil {
				return op, e
			}
		}
		for i := 0; int64(i)*PartSize < op.Size; i++ {
			part, e := os.Open(filepath.Join(dir, fmt.Sprintf("%08d", i)))
			if e != nil {
				return op, code("missing_parts")
			}
			_, e = io.Copy(f, part)
			part.Close()
			if e != nil {
				return op, code("disk_write_failed")
			}
		}
		// Pre-publish gate: decrypt the assembled object, keep the plaintext
		// digest and sniff the MIME type from the decrypted head.
		if _, e = f.Seek(0, 0); e != nil {
			return op, code("disk_write_failed")
		}
		var opener *contentOpener
		if opener, e = newContentOpener(aead, f, op.Size); e != nil {
			return op, e
		}
		head := &prefixWriter{limit: 512}
		if _, e = io.Copy(io.MultiWriter(h, head), opener); e != nil {
			return op, e
		}
		if hex.EncodeToString(h.Sum(nil)) != op.SHA256 {
			return op, code("content_mismatch")
		}
		mime = detectMIME(http.DetectContentType(head.buf), op.Path)
		if info, statErr := f.Stat(); statErr != nil {
			return op, code("disk_write_failed")
		} else {
			objectSize = info.Size()
		}
	} else {
		for i := 0; int64(i)*PartSize < op.Size; i++ {
			part, e := os.Open(filepath.Join(dir, fmt.Sprintf("%08d", i)))
			if e != nil {
				return op, code("missing_parts")
			}
			n, e := io.Copy(io.MultiWriter(f, h), part)
			part.Close()
			if e != nil || n <= 0 {
				return op, code("disk_write_failed")
			}
		}
		if hex.EncodeToString(h.Sum(nil)) != op.SHA256 {
			return op, code("content_mismatch")
		}
		f.Seek(0, 0)
		header := make([]byte, 512)
		n, _ := f.Read(header)
		mime = detectMIME(http.DetectContentType(header[:n]), op.Path)
	}
	f.Seek(0, 0)
	if e = s.DB.Model(&op).Updates(map[string]any{"status": "verifying", "error": ""}).Error; e != nil {
		return op, code("database_unavailable")
	}
	objectKey := s.blobObjectKey(blobScope(sp), op.SHA256)
	meter := &insights.ObservedReadSeeker{ReadSeeker: f}
	e = storage.Put(ctx, objectKey, meter, objectSize)
	s.observed(p, op, "server_to_storage", "transfer", meter.Bytes)
	if e != nil {
		s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": "storage_write_failed"})
		return op, e
	}
	// Verify bytes from the selected store before publishing metadata; for
	// encrypted spaces the read-back is decrypted and digest-checked.
	content, e := storage.Get(ctx, objectKey)
	if e != nil {
		return op, e
	}
	if s.Insights != nil {
		content = s.Insights.Observe(content, insights.Fact{Dimensions: s.dimensions(p, op), Metric: "observed_bytes", Direction: "storage_to_server", Purpose: "verification"})
	}
	verify := sha256.New()
	if encrypted {
		var opener *contentOpener
		opener, e = newContentOpener(aead, content, op.Size)
		if e == nil {
			_, e = io.Copy(verify, opener)
		}
		content.Close()
		if e != nil || hex.EncodeToString(verify.Sum(nil)) != op.SHA256 {
			return op, code("content_mismatch")
		}
	} else {
		var readN int64
		readN, e = io.Copy(verify, io.LimitReader(content, op.Size+1))
		content.Close()
		if e != nil || readN != op.Size || hex.EncodeToString(verify.Sum(nil)) != op.SHA256 {
			return op, code("content_mismatch")
		}
	}
	return s.publishUploadAuthorized(p, op, encrypted, mime, dir, workspace)
}
func (s *Service) Cancel(p Principal, id string) (Operation, error) {
	if _, e := s.cancellableOperation(p, id); e != nil {
		return Operation{}, e
	}
	s.interruptOperation(id)
	unlock := s.opLock(id)
	defer unlock()
	op, e := s.cancellableOperation(p, id)
	if e != nil {
		return op, e
	}
	if op.Status == "completed" || op.Status == "cancelled" {
		return op, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var sp Space
		if tx.First(&sp, "id = ? AND owner = ?", op.SpaceID, p.dataOwner()).Error != nil {
			return code("not_found")
		}
		// Only createUpload-created kinds (upload, archive, copy) hold a
		// reservation; directory and export kinds never reserved, so
		// cancelling them must not demand or release one.
		reserves := op.Kind == "upload" || op.Kind == "archive" || op.Kind == "copy"
		if reserves && sp.Reserved < op.Size {
			return code("database_unavailable")
		}
		if reserves {
			if e := tx.Model(&sp).Update("reserved", sp.Reserved-op.Size).Error; e != nil {
				return e
			}
		}
		op.Status = "cancelled"
		op.Updated = time.Now().UTC()
		return tx.Save(&op).Error
	})
	if e == nil {
		_ = os.RemoveAll(filepath.Join(s.Config.State, "uploads", op.ID))
		// A cancelled operation can never deliver bytes again, so its
		// reservation is released with an explicit failed outcome.
		_ = s.reportServiceAdmission(op, "failed", 0)
		s.finishBatchItem(op, "cancelled", "operation_cancelled")
	}
	return op, e
}
func (s *Service) Download(ctx context.Context, p Principal, id, version string) (File, Version, io.ReadCloser, error) {
	f, e := s.file(p, id, "read")
	if e != nil {
		return f, Version{}, nil, e
	}
	if f.Trashed {
		return f, Version{}, nil, code("file_trashed")
	}
	if version == "" {
		version = f.VersionID
	}
	var v Version
	if s.DB.First(&v, "id = ? AND owner = ? AND file_id = ?", version, p.dataOwner(), id).Error != nil {
		return f, v, nil, code("not_found")
	}
	return s.downloadResolvedVersion(ctx, p, f, v, false)
}

func (s *Service) downloadResolvedVersion(ctx context.Context, p Principal, f File, v Version, workspace bool) (File, Version, io.ReadCloser, error) {
	guard := func(body io.ReadCloser) io.ReadCloser {
		if workspace {
			return &workspaceVersionReader{ReadCloser: body, service: s, principal: p, fileID: f.ID, versionID: v.ID}
		}
		return s.guardedStream(p, f, body)
	}
	store, e := s.storageForSpace(p.dataOwner(), f.SpaceID, v.ConnectionID)
	if e != nil {
		return f, v, nil, e
	}
	body, e := store.Get(ctx, v.ObjectKey)
	if e != nil {
		if v.BlobSHA256 != "" {
			return f, v, nil, code("blob_unavailable")
		}
		return f, v, nil, e
	}
	if s.Insights != nil {
		body = s.Insights.Observe(body, insights.Fact{Dimensions: s.dimensions(p, Operation{SpaceID: f.SpaceID, ConnectionID: v.ConnectionID, FileID: f.ID}), Metric: "observed_bytes", Direction: "storage_to_server", Purpose: "download"})
	}
	var sp Space
	if err := s.DB.First(&sp, "id = ? AND owner = ?", f.SpaceID, p.dataOwner()).Error; err != nil {
		body.Close()
		return f, v, nil, code("database_unavailable")
	}
	if sp.Encrypted {
		aead, e := s.spaceContentAEAD(sp)
		if e != nil {
			body.Close()
			return f, v, nil, e
		}
		opener, e := newContentOpener(aead, body, v.Size)
		if e != nil {
			body.Close()
			return f, v, nil, e
		}
		return f, v, guard(newVerifyReader(opener, v.SHA256)), nil
	}
	return f, v, guard(body), nil
}

// prefixWriter keeps the first limit bytes passed through Write.
type prefixWriter struct {
	buf   []byte
	limit int
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	if len(p.buf) < p.limit {
		n := min(p.limit-len(p.buf), len(b))
		p.buf = append(p.buf, b[:n]...)
	}
	return len(b), nil
}

// ExportSpacePlaintext is the encrypted-space escape hatch: it writes the
// current plaintext version of every live file into outDir so an encrypted
// deployment can be exported before rollback. Trashed files are skipped and
// reported.
func (s *Service) ExportSpacePlaintext(ctx context.Context, spaceID, outDir string) (map[string]any, error) {
	var sp Space
	if s.DB.First(&sp, "id = ?", spaceID).Error != nil {
		return nil, code("not_found")
	}
	if !sp.Encrypted {
		return nil, code("space_not_encrypted")
	}
	if e := os.MkdirAll(outDir, 0700); e != nil {
		return nil, code("disk_write_failed")
	}
	p := Principal{UserID: sp.Owner}
	var files []File
	s.DB.Where("space_id = ? AND owner = ?", spaceID, sp.Owner).Find(&files)
	exported, skipped := 0, 0
	var written int64
	for _, file := range files {
		if file.Trashed {
			skipped++
			continue
		}
		_, _, body, e := s.Download(ctx, p, file.ID, "")
		if e != nil {
			skipped++
			continue
		}
		target := filepath.Join(outDir, filepath.FromSlash(file.Path))
		if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			body.Close()
			return nil, code("disk_write_failed")
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			body.Close()
			return nil, code("disk_write_failed")
		}
		n, e := io.Copy(out, body)
		out.Close()
		body.Close()
		if e != nil {
			skipped++
			continue
		}
		exported++
		written += n
	}
	return map[string]any{"space_id": spaceID, "files_exported": exported, "files_skipped": skipped, "bytes_written": written, "output_dir": outDir}, nil
}
func (s *Service) Trash(p Principal, id string, restore bool) error {
	f, e := s.file(p, id, "delete")
	if e != nil {
		return e
	}
	if e := s.requireAudit(p, "files.trash_restore", id); e != nil {
		return e
	}
	defer s.CaptureUsage(p.dataOwner())
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if e := teamWritable(tx, f.SpaceID); e != nil {
			return e
		}
		// The event revision and File.Updated move together: the backfill of
		// a projection rebuild derives revisions from File.Updated, so both
		// materialization paths must agree on the same monotonic source.
		rev := fileRevision(f.Updated)
		if next := time.Now().UTC().UnixNano(); next > rev {
			rev = next
		} else {
			rev++
		}
		updated := time.Unix(0, rev).UTC()
		if restore {
			res := tx.Model(&File{}).Where("id = ? AND trashed = ?", f.ID, true).Updates(map[string]any{"trashed": false, "trashed_at": time.Time{}, "updated": updated})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				var n int64
				tx.Model(&File{}).Where("id = ?", f.ID).Count(&n)
				if n == 0 {
					// A concurrent purge deleted the row between the read
					// and this update; the restore must not report success.
					return code("not_found")
				}
				// Already restored: idempotent success.
			}
			return s.recordFileEvent(tx, p.dataOwner(), f.ID, rev, false, map[string]any{"path": f.Path, "restored": true})
		}
		if e := tx.Model(&f).Updates(map[string]any{"trashed": true, "trashed_at": updated, "updated": updated}).Error; e != nil {
			return e
		}
		// A trash is a tombstone in the same transaction: the projection
		// must never resurrect a file the business layer hid.
		return s.recordFileEvent(tx, p.dataOwner(), f.ID, rev, true, map[string]any{"path": f.Path, "trashed": true})
	})
}
func (s *Service) Purge(ctx context.Context, p Principal, id string) error {
	if p.Agent != nil {
		return code("forbidden")
	}
	f, e := s.file(p, id, "delete")
	if e != nil {
		return e
	}
	if !f.Trashed {
		return code("trash_required")
	}
	if e := s.requireAudit(p, "files.purge", id); e != nil {
		return e
	}
	defer s.CaptureUsage(p.dataOwner())
	type legacyObject struct {
		store Storage
		key   string
	}
	legacy := []legacyObject{}
	s.mu.Lock()
	e = func() error {
		use, err := s.fileUse(p, f)
		if err != nil {
			return err
		}
		if !use["permanent_delete_allowed"].(bool) {
			return code("file_in_use")
		}
		if e := s.allow(p, f.SpaceID, f.Path, "delete"); e != nil {
			return e
		}
		var versions []Version
		s.DB.Where("file_id = ? AND owner = ?", id, p.dataOwner()).Find(&versions)
		var total int64
		var blobRefs []string
		for _, v := range versions {
			if v.BlobSHA256 != "" {
				// Shared blob objects survive purge; only the reference is
				// released, and a zero count never deletes by itself.
				blobRefs = append(blobRefs, blobID(v.Owner, blobScopeForVersion(s, v), v.BlobSHA256))
				total += v.Size
				continue
			}
			st, e := s.storageForSpace(p.dataOwner(), f.SpaceID, v.ConnectionID)
			if e != nil {
				return e
			}
			legacy = append(legacy, legacyObject{store: st, key: v.ObjectKey})
			total += v.Size
		}
		return s.DB.Transaction(func(tx *gorm.DB) error {
			// Protection and deletion decide on the same rows in one
			// transaction: an active hold on any version of this file skips the
			// purge and names the protecting source.
			now := time.Now().UTC()
			var heldVersions []Version
			if e := tx.Where("file_id = ? AND owner = ?", id, p.dataOwner()).Find(&heldVersions).Error; e != nil {
				return e
			}
			for _, v := range heldVersions {
				hold, err := activeHoldForVersion(tx, id, v.ID, now)
				if err != nil {
					return err
				}
				if hold != nil {
					return code("version_protected")
				}
			}
			// Phase one of the two-phase purge: the durable in-progress mark
			// commits with the hold check, so a later hold creation observes
			// purge_in_progress instead of promising protection over bytes that
			// are already being released.
			if e := tx.Model(&File{}).Where("id = ? AND owner = ? AND trashed = ?", id, p.dataOwner(), true).Update("purge_started_at", now).Error; e != nil {
				return e
			}
			// A restore committing after the pre-check must win: the delete
			// is conditional on the file still being trashed, so a live
			// file is never purged out from under its owner.
			res := tx.Where("id = ? AND owner = ? AND trashed = ?", id, p.dataOwner(), true).Delete(&File{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				var n int64
				tx.Model(&File{}).Where("id = ? AND owner = ?", id, p.dataOwner()).Count(&n)
				if n == 0 {
					return code("not_found")
				}
				return code("trash_required")
			}
			var sp Space
			if tx.First(&sp, "id = ? AND owner = ?", f.SpaceID, p.dataOwner()).Error != nil {
				return code("not_found")
			}
			for _, ref := range blobRefs {
				if e := releaseBlob(tx, ref); e != nil {
					return e
				}
			}
			if e := tx.Where("file_id = ? AND owner = ?", id, p.dataOwner()).Delete(&Version{}).Error; e != nil {
				return e
			}
			if e := tx.Model(&sp).Update("used", sp.Used-total).Error; e != nil {
				return e
			}
			// The purge tombstone joins the delete transaction so the
			// projection drops the resource exactly when the business row is
			// gone and never before. The revision stays strictly above every
			// revision the live file ever published, so a rebuild backfill
			// cannot order a live row above the tombstone.
			rev := fileRevision(f.Updated)
			if next := time.Now().UTC().UnixNano(); next > rev {
				rev = next
			} else {
				rev++
			}
			return s.recordFileEvent(tx, p.dataOwner(), f.ID, rev, true, map[string]any{"path": f.Path, "purged": true})
		})
	}()
	s.mu.Unlock()
	if e != nil {
		return e
	}
	// Legacy un-deduplicated objects are deleted only after the row
	// transaction commits and outside s.mu: a failure here orphans a sweepable
	// object instead of leaving metadata rows pointing at deleted storage.
	for _, o := range legacy {
		if e := o.store.Delete(ctx, o.key); e != nil {
			return e
		}
	}
	return nil
}

// RepairParts reconciles restored metadata with local staged bytes. A metadata-
// only restore must request the original source instead of claiming lost parts.
func (s *Service) repairParts() error {
	var operations []Operation
	if e := s.DB.Where("kind IN ? AND status NOT IN ?", []string{"upload", "import", "archive"}, []string{"completed", "cancelled"}).Find(&operations).Error; e != nil {
		return e
	}
	for _, op := range operations {
		var parts []Part
		s.DB.Where("operation_id = ?", op.ID).Find(&parts)
		var available int64
		for _, part := range parts {
			info, e := os.Lstat(filepath.Join(s.Config.State, "uploads", op.ID, fmt.Sprintf("%08d", part.Index)))
			if e != nil || !info.Mode().IsRegular() || info.Size() != part.Size {
				if e = s.DB.Delete(&part).Error; e != nil {
					return e
				}
			} else {
				// Staged bytes may be ciphertext (encrypted spaces); the
				// transferred counter is logical plaintext, derived from the
				// deterministic part layout of the operation.
				available += min(PartSize, op.Size-int64(part.Index)*PartSize)
			}
		}
		if available != op.Transferred {
			if e := s.DB.Model(&op).Updates(map[string]any{"transferred": available, "status": "interrupted", "error": "original_source_required"}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

// publishUpload runs the shared completion publish: final authorization
// re-check, the metadata transaction (file, version, blob claim, quota
// settle, outbox event) and the post-commit hooks for both the legacy
// assembled path and the streaming batch path.
func (s *Service) publishUpload(p Principal, op Operation, encrypted bool, mime string, dir string) (Operation, error) {
	return s.publishUploadAuthorized(p, op, encrypted, mime, dir, false)
}

func (s *Service) publishUploadAuthorized(p Principal, op Operation, encrypted bool, mime string, dir string, workspace bool) (Operation, error) {
	if workspace {
		if _, _, err := s.workspaceOperationOn(s.DB, p, op.ID, time.Now().UTC()); err != nil {
			return op, err
		}
	} else {
		if op.Purpose != "" || op.WorkspaceGrantID != "" {
			return op, code("forbidden")
		}
		if e := s.allow(p, op.SpaceID, op.Path, "write"); e != nil {
			return op, e
		}
	}
	if op.ExpectedVersion != "" && s.governanceActive() {
		if e := s.allow(p, op.SpaceID, op.Path, "overwrite"); e != nil {
			return op, e
		}
	}
	var sp Space
	if s.DB.First(&sp, "id = ? AND owner = ?", op.SpaceID, p.dataOwner()).Error != nil {
		return op, code("not_found")
	}
	objectKey := s.blobObjectKey(blobScope(sp), op.SHA256)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if workspace {
			current, sub, err := s.workspaceOperationOn(tx, p, op.ID, time.Now().UTC())
			if err != nil {
				return err
			}
			if sub.State != "uploading" || current.FileID != "" || current.VersionID != op.VersionID || current.Path != op.Path || current.SHA256 != op.SHA256 || current.Size != op.Size || current.ConnectionID != op.ConnectionID {
				return code("invalid_operation")
			}
		} else {
			var persisted Operation
			if err := tx.Where("id = ? AND owner = ?", op.ID, p.dataOwner()).First(&persisted).Error; err != nil {
				return code("database_unavailable")
			}
			if persisted.Purpose != "" || persisted.WorkspaceGrantID != "" {
				return code("forbidden")
			}
			if e := s.allowOn(tx, p, op.SpaceID, op.Path, "write"); e != nil {
				return e
			}
		}
		if op.ExpectedVersion != "" && s.governanceActive() {
			if e := s.allowOn(tx, p, op.SpaceID, op.Path, "overwrite"); e != nil {
				return e
			}
		}
		if e := teamWritable(tx, op.SpaceID); e != nil {
			return e
		}
		var sp Space
		if tx.First(&sp, "id = ? AND owner = ?", op.SpaceID, p.dataOwner()).Error != nil {
			return code("not_found")
		}
		var conn Connection
		if tx.First(&conn, "id = ? AND disabled = ?", op.ConnectionID, false).Error != nil {
			return code("storage_disconnected")
		}
		var file File
		// Archive and copy-with-rename resolved their path at creation time;
		// by completion the name may have been taken. Rename ops re-resolve
		// here so a late taker cannot turn rename semantics into a hard
		// name_conflict failure (or land a file on a directory's path).
		if (op.Kind == "archive" || op.Kind == "copy") && op.RenameOnConflict && op.FileID == "" {
			if e := s.allowOn(tx, p, op.SpaceID, op.Path, "write"); e != nil {
				return e
			}
			taken, e := pathTaken(tx, op.SpaceID, op.Path)
			if e != nil {
				return e
			}
			if taken {
				var err error
				op.Path, err = archivePath(tx, op.SpaceID, op.Path)
				if err != nil {
					return err
				}
				if err = s.allow(p, op.SpaceID, op.Path, "write"); err != nil {
					return err
				}
			}
		}
		if op.FileID != "" {
			if tx.First(&file, "id = ? AND owner = ? AND trashed = ?", op.FileID, p.dataOwner(), false).Error != nil || file.VersionID != op.ExpectedVersion {
				return code("version_conflict")
			}
		} else {
			file = File{ReviewPending: op.ReviewPending, ID: ID(), Owner: p.dataOwner(), SpaceID: op.SpaceID, Path: op.Path}
			if tx.Create(&file).Error != nil {
				return code("name_conflict")
			}
		}
		blob, e := claimBlob(tx, Blob{ID: blobID(p.dataOwner(), blobScope(sp), op.SHA256), SHA256: op.SHA256, Owner: p.dataOwner(), SpaceID: blobScope(sp), ConnectionID: op.ConnectionID, ObjectKey: objectKey, Size: op.Size, MIME: mime, Encrypted: encrypted})
		if e != nil {
			return e
		}
		v := Version{Caller: op.Caller, TaskRef: op.TaskRef, ID: op.VersionID, Owner: p.dataOwner(), FileID: file.ID, ConnectionID: op.ConnectionID, ObjectKey: blob.ObjectKey, BlobSHA256: op.SHA256, SHA256: op.SHA256, Size: op.Size, MIME: mime, Created: time.Now().UTC()}
		if tx.Create(&v).Error != nil {
			return code("database_unavailable")
		}
		file.ReviewPending = op.ReviewPending
		file.Caller, file.TaskRef = op.Caller, op.TaskRef
		file.VersionID = v.ID
		file.Size = v.Size
		file.MIME = mime
		file.Updated = v.Created
		if tx.Save(&file).Error != nil {
			return code("database_unavailable")
		}
		op.FileID = file.ID
		op.Status = "completed"
		op.MIME = mime
		op.Error = ""
		op.Updated = v.Created
		if e := tx.Save(&op).Error; e != nil {
			return e
		}
		// The completed upload settles its creation-time reservation: logical
		// usage moves and the reservation is released in the same transaction.
		if e := tx.Model(&sp).Updates(map[string]any{"reserved": sp.Reserved - op.Size, "used": sp.Used + op.Size}).Error; e != nil {
			return e
		}
		if workspace {
			if err := s.finishWorkspaceSubmissionOn(tx, p, op.ID, v.Created); err != nil {
				return err
			}
		}
		if err := s.finishWorkspaceArchiveOn(tx, p, op, v.Created); err != nil {
			return err
		}
		// The file_catalogue event and its apply job join the business
		// commit: a rolled-back completion never publishes an event, and a
		// committed file is never missing from the outbox.
		return s.recordFileEvent(tx, p.dataOwner(), file.ID, fileRevision(file.Updated), false, map[string]any{"path": file.Path, "size_bytes": file.Size, "mime_type": file.MIME, "trashed": file.Trashed})
	})
	if e != nil {
		s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e)})
		s.finishBatchItem(op, "failed", safeCode(e))
	}
	if e == nil {
		_ = os.RemoveAll(dir)
		// Deliveries settle the reservation immediately; locally failed
		// operations remain recoverable, so their reservation is held.
		_ = s.reportServiceAdmission(op, "delivered", op.Size)
		s.finishBatchItem(op, "completed", "")
	}
	if e == nil && s.Insights != nil {
		_ = s.Insights.Record(insights.Fact{ID: "completed-" + op.ID, Dimensions: s.dimensions(p, op), Metric: "effective_bytes", Direction: "ingest", Purpose: op.Kind, Bytes: op.Size})
		s.CaptureUsage(p.dataOwner())
	}
	return op, e
}

// seekableCountingReader keeps the S3 signing path rewindable while
// counting only the payload pass: a rewind for hashing resets the counter so
// observed bytes never double-count.
type seekableCountingReader struct {
	io.Reader
	src   *partReader
	count int64
}

func (s *seekableCountingReader) Read(p []byte) (int, error) {
	n, e := s.Reader.Read(p)
	s.count += int64(n)
	return n, e
}

func (s *seekableCountingReader) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		s.count = 0
	}
	return s.src.Seek(offset, whence)
}

// streamUploadToObject reassembles the verified staged client parts
// directly into the target object and reads the stored bytes back for the
// final digest check. The local digest and MIME head pass streams the staged
// parts once; the payload pass rewinds through whole-part seeks, so memory
// stays bounded regardless of file size and no assembled local copy exists.
func (s *Service) streamUploadToObject(ctx context.Context, p Principal, op Operation, sp Space, storage Storage) (string, int64, error) {
	reader := s.streamStagedParts(op)
	defer reader.Close()
	h := sha256.New()
	head := &prefixWriter{limit: 512}
	if _, e := io.Copy(io.MultiWriter(h, head), reader); e != nil {
		return "", 0, code("missing_parts")
	}
	if hex.EncodeToString(h.Sum(nil)) != op.SHA256 {
		s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": "content_mismatch"})
		return "", 0, code("content_mismatch")
	}
	if e := s.DB.Model(&op).Updates(map[string]any{"status": "verifying", "error": ""}).Error; e != nil {
		return "", 0, code("database_unavailable")
	}
	if _, e := reader.Seek(0, io.SeekStart); e != nil {
		return "", 0, e
	}
	objectKey := s.blobObjectKey(blobScope(sp), op.SHA256)
	meter := &seekableCountingReader{Reader: reader, src: reader}
	if e := storage.Put(ctx, objectKey, meter, op.Size); e != nil {
		s.DB.Model(&op).Updates(map[string]any{"status": "failed", "error": "storage_write_failed"})
		return "", 0, e
	}
	s.observed(p, op, "server_to_storage", "transfer", meter.count)
	content, e := storage.Get(ctx, objectKey)
	if e != nil {
		return "", 0, e
	}
	if s.Insights != nil {
		content = s.Insights.Observe(content, insights.Fact{Dimensions: s.dimensions(p, op), Metric: "observed_bytes", Direction: "storage_to_server", Purpose: "verification"})
	}
	verify := sha256.New()
	var readN int64
	readN, e = io.Copy(verify, io.LimitReader(content, op.Size+1))
	content.Close()
	if e != nil || readN != op.Size || hex.EncodeToString(verify.Sum(nil)) != op.SHA256 {
		return "", 0, code("content_mismatch")
	}
	return detectMIME(http.DetectContentType(head.buf), op.Path), op.Size, nil
}
