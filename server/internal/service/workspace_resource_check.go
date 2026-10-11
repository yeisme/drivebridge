package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const resourceIntegrityChecker = contract.ResourceIntegrityChecker
const resourceIntegrityBytes int64 = 2 << 20

// Passed covers only fixed-byte integrity and UTF-8 without NUL. It is not
// skill conformance, prompt trust, malware analysis or permission to execute.
type ResourceIntegrityReceipt = contract.ResourceIntegrityReceipt

type resourceIntegrityTarget struct {
	FileID       string
	VersionID    string
	SpaceID      string
	Path         string
	SnapshotPath string
	Size         int64
	SHA256       string
	ConnectionID string
	ObjectKey    string
}

func (s *Service) resourceIntegrityTargetsOn(tx *gorm.DB, p Principal, id string, revision uint64) (WorkspaceResourceDescriptor, []resourceIntegrityTarget, string, error) {
	var row WorkspaceResourceDescriptor
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return row, nil, "", err
	}
	if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&row).Error; err != nil {
		return row, nil, "", resourceReadError(err)
	}
	if revision == 0 || row.Revision != revision {
		return row, nil, "", code("revision_conflict")
	}
	if _, err := s.resourceViewOn(tx, p, row); err != nil {
		return row, nil, "", err
	}
	var entries []CollectionSnapshotEntry
	if row.SnapshotID != nil {
		if err := tx.Where("snapshot_id = ?", *row.SnapshotID).Order("id").Limit(101).Find(&entries).Error; err != nil {
			return row, nil, "", code("database_unavailable")
		}
	} else {
		entries = []CollectionSnapshotEntry{{FileID: *row.FileID, VersionID: *row.VersionID}}
	}
	if len(entries) == 0 {
		return row, nil, "", code("unsupported_action")
	}
	if len(entries) > 100 {
		return row, nil, "", code("query_budget_exceeded")
	}
	total := int64(0)
	targets := make([]resourceIntegrityTarget, 0, len(entries))
	for _, entry := range entries {
		var file File
		if err := tx.Where("id = ? AND owner = ?", entry.FileID, p.UserID).First(&file).Error; err != nil {
			return row, nil, "", resourceReadError(err)
		}
		var version Version
		if err := tx.Where("id = ? AND owner = ? AND file_id = ?", entry.VersionID, p.UserID, file.ID).First(&version).Error; err != nil {
			return row, nil, "", resourceReadError(err)
		}
		if version.Size < 0 || version.Size > resourceIntegrityBytes-total {
			return row, nil, "", code("query_budget_exceeded")
		}
		hash, err := hex.DecodeString(version.SHA256)
		if err != nil || len(hash) != 32 {
			return row, nil, "", code("dependency_unavailable")
		}
		total += version.Size
		targets = append(targets, resourceIntegrityTarget{FileID: file.ID, VersionID: version.ID, SpaceID: file.SpaceID, Path: file.Path, SnapshotPath: entry.Path, Size: version.Size, SHA256: version.SHA256, ConnectionID: version.ConnectionID, ObjectKey: version.ObjectKey})
	}
	// Private storage identity participates only in a digest; neither paths,
	// storage keys nor file contents enter a check receipt or error.
	encoded, err := json.Marshal(struct {
		Descriptor WorkspaceResourceDescriptor
		Targets    []resourceIntegrityTarget
	}{row, targets})
	if err != nil {
		return row, nil, "", code("dependency_unavailable")
	}
	return row, targets, Hash(string(encoded)), nil
}

func resourceIntegrityReceipt(row WorkspaceResourceCheck) ResourceIntegrityReceipt {
	return ResourceIntegrityReceipt{CheckID: row.ID, DescriptorID: row.DescriptorID, DescriptorRevision: row.DescriptorRevision, CheckerVersion: row.CheckerVersion, CheckScope: contract.ResourceIntegrityScope, State: row.State, CheckedAt: row.CreatedAt}
}

func (s *Service) CheckResourceIntegrity(ctx context.Context, p Principal, id string, revision uint64) (ResourceIntegrityReceipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var targets []resourceIntegrityTarget
	var digest string
	var prior WorkspaceResourceCheck
	replay := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, list, targetDigest, err := s.resourceIntegrityTargetsOn(tx, p, id, revision)
		if err != nil {
			return err
		}
		targets, digest = list, targetDigest
		err = tx.Where("owner = ? AND tenant_id = ? AND descriptor_id = ? AND descriptor_revision = ? AND checker_version = ?", p.UserID, p.TenantID, id, revision, resourceIntegrityChecker).First(&prior).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return code("database_unavailable")
		}
		if prior.TargetDigest != digest || prior.CreatedAt.IsZero() || (prior.State != "passed" && prior.State != "failed") {
			return code("revision_conflict")
		}
		replay = true
		return nil
	})
	if err != nil {
		return ResourceIntegrityReceipt{}, err
	}
	if replay {
		return resourceIntegrityReceipt(prior), nil
	}
	state := "passed"
	for _, target := range targets {
		if ctx.Err() != nil {
			return ResourceIntegrityReceipt{}, ctx.Err()
		}
		file, version, body, err := s.Download(ctx, p, target.FileID, target.VersionID)
		if err != nil {
			return ResourceIntegrityReceipt{}, err
		}
		if file.ID != target.FileID || file.SpaceID != target.SpaceID || version.ID != target.VersionID || version.Size != target.Size || version.SHA256 != target.SHA256 {
			body.Close()
			return ResourceIntegrityReceipt{}, code("query_changed")
		}
		data, readErr := io.ReadAll(io.LimitReader(body, target.Size+1))
		closeErr := body.Close()
		if readErr != nil || closeErr != nil {
			return ResourceIntegrityReceipt{}, code("dependency_unavailable")
		}
		if int64(len(data)) != target.Size || Hash(string(data)) != target.SHA256 {
			return ResourceIntegrityReceipt{}, code("dependency_unavailable")
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			state = "failed"
		}
	}
	if ctx.Err() != nil {
		return ResourceIntegrityReceipt{}, ctx.Err()
	}
	var result WorkspaceResourceCheck
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, _, current, err := s.resourceIntegrityTargetsOn(tx, p, id, revision)
		if err != nil {
			return err
		}
		if current != digest {
			return code("query_changed")
		}
		candidate := WorkspaceResourceCheck{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, DescriptorID: id, DescriptorRevision: revision, CheckerVersion: resourceIntegrityChecker, State: state, TargetDigest: digest, CreatedAt: time.Now().UTC()}
		written := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if written.Error != nil {
			return code("database_unavailable")
		}
		if written.RowsAffected == 1 {
			if err := tx.Create(&Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.resource.check", Resource: id, Outcome: state, At: candidate.CreatedAt}).Error; err != nil {
				return code("audit_unavailable")
			}
			result = candidate
			return nil
		}
		if err := tx.Where("owner = ? AND tenant_id = ? AND descriptor_id = ? AND descriptor_revision = ? AND checker_version = ?", p.UserID, p.TenantID, id, revision, resourceIntegrityChecker).First(&result).Error; err != nil {
			return code("database_unavailable")
		}
		if result.TargetDigest != digest || result.State != state || result.CreatedAt.IsZero() {
			return code("revision_conflict")
		}
		return nil
	})
	if err != nil {
		return ResourceIntegrityReceipt{}, err
	}
	return resourceIntegrityReceipt(result), nil
}
