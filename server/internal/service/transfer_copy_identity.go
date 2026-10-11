package service

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

// Resolve allocation before checking target occupancy: a previously published
// result is not a new conflict. Old unlinked allocations are adopted only when
// their complete origin fingerprint matches, including the requested target.
func (s *Service) transferCopyIdentity(p Principal, plan TransferPlan, item TransferItem) (string, *Operation, error) {
	request := ArchiveRequest{SourceFileID: item.SourceFileID, SourceVersionID: item.SourceVersionID, TargetSpaceID: plan.TargetSpaceID, TargetPath: item.TargetPath, ExpectedSourceVersion: item.SourceVersionID}
	raw, err := json.Marshal(ingestOrigin{Kind: "archive", Source: request, AutoRename: true})
	if err != nil {
		return "", nil, err
	}
	fingerprint := Hash("archive:" + string(raw))
	key := "transfer-copy:" + Hash(plan.ID+"\x00"+item.ItemKey)
	matches := func(op Operation) bool {
		return op.Kind == "archive" && op.Fingerprint == fingerprint && op.ArchiveFileID == item.SourceFileID && op.ArchiveVersionID == item.SourceVersionID && op.ArchiveHeadVersion == item.SourceVersionID && op.SpaceID == plan.TargetSpaceID && op.Purpose == "" && op.WorkspaceGrantID == ""
	}
	if item.CopyOperationID != "" {
		var op Operation
		if err := s.DB.Where("id = ? AND owner = ?", item.CopyOperationID, p.dataOwner()).First(&op).Error; err != nil {
			return "", nil, code("database_unavailable")
		}
		if !matches(op) {
			return "", nil, code("idempotency_conflict")
		}
		return op.Key, &op, nil
	}
	for _, candidate := range []string{key, item.ItemKey} {
		var op Operation
		err := s.DB.Where("owner = ? AND key = ?", p.dataOwner(), candidate).First(&op).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return "", nil, code("database_unavailable")
		}
		if matches(op) {
			return candidate, &op, nil
		}
		if candidate == key {
			return "", nil, code("idempotency_conflict")
		}
		// Another plan's legacy key must not suppress this plan's own allocation.
	}
	return key, nil, nil
}
