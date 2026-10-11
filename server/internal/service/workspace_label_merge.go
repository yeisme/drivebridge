package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"
	"strings"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func isLabelMergePlan(plan OrganizationPlan) bool {
	for _, action := range decodePlanActions(plan.Actions) { if action.Kind == "merge_label" { return true } }
	return false
}

func labelMergeRequest(plan OrganizationPlan) (contract.LabelMergePreview, error) {
	actions := decodePlanActions(plan.Actions)
	var q contract.LabelMergePreview
	if len(actions) != 1 || actions[0].Kind != "merge_label" || json.Unmarshal([]byte(actions[0].Value), &q) != nil || !validLabelMerge(q) { return q, code("dependency_unavailable") }
	return q, nil
}

func validLabelMerge(q contract.LabelMergePreview) bool {
	return q.Namespace == "manual" && propertyKey.MatchString(q.SourceKey) && propertyKey.MatchString(q.TargetKey) && q.SourceKey != q.TargetKey && q.ExpectedRevision > 0 && q.ExpectedRevision <= 9007199254740991 && q.SourceRevision > 0 && q.TargetRevision > 0 && len(q.Refs) > 0 && len(q.Refs) <= 200
}

func (s *Service) labelMergeCatalogOn(tx *gorm.DB, p Principal, id string, q contract.LabelMergePreview) error {
	catalog, err := s.labelDefinitionsOn(tx, p, id)
	if err != nil { return err }
	if catalog.Revision != q.ExpectedRevision { return code("revision_conflict") }
	source, target := false, false
	for _, row := range catalog.Definitions {
		if row.Key == q.SourceKey { source = row.Revision == q.SourceRevision }
		if row.Key == q.TargetKey { target = row.Revision == q.TargetRevision }
	}
	if !source || !target { return code("revision_conflict") }
	return nil
}

func (s *Service) labelMergeFileOn(tx *gorm.DB, p Principal, spaceID, id, version string, write bool) (File, error) {
	var file File
	query := tx
	if write { query = query.Clauses(clause.Locking{Strength: "UPDATE"}) }
	if err := query.Where("id = ? AND owner = ? AND space_id = ?", id, p.UserID, spaceID).First(&file).Error; err != nil { return file, resourceReadError(err) }
	if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) { return file, code("not_found") }
	if err := s.allowOn(tx, p, spaceID, file.Path, "read"); err != nil { return file, err }
	if write {
		if err := s.allowOn(tx, p, spaceID, file.Path, "write"); err != nil { return file, err }
		if file.VersionID != version { return file, code("version_conflict") }
	}
	var fixed Version
	if err := tx.Where("id = ? AND file_id = ? AND owner = ?", version, id, p.UserID).First(&fixed).Error; err != nil { return file, resourceReadError(err) }
	return file, nil
}

func labelMergeAssignmentsOn(tx *gorm.DB, owner, fileID string) ([]FileLabelAssignment, string, error) {
	var rows []FileLabelAssignment
	if err := tx.Where("owner = ? AND file_id = ?", owner, fileID).Order("namespace, label, id").Limit(601).Find(&rows).Error; err != nil { return nil, "", code("database_unavailable") }
	if len(rows) > 600 { return nil, "", code("query_budget_exceeded") }
	body, err := json.Marshal(rows)
	if err != nil { return nil, "", code("dependency_unavailable") }
	return rows, Hash(string(body)), nil
}

// Read and recovery reauthorize all bounded members before revealing impacts.
func (s *Service) labelMergePlanAuthorizeOn(tx *gorm.DB, p Principal, plan OrganizationPlan, write bool) error {
	if plan.Owner != p.UserID || plan.TenantID != p.TenantID { return code("not_found") }
	if write && !s.Config.LabelManagement { return code("unsupported_action") }
	if err := s.propertySpaceOn(tx, p, plan.SpaceID, write); err != nil { return err }
	q, err := labelMergeRequest(plan)
	if err != nil { return err }
	if write { if err = s.labelMergeCatalogOn(tx, p, plan.SpaceID, q); err != nil { return err } }
	var items []OrganizationPlanItem
	if err = tx.Where("plan_id = ?", plan.ID).Limit(201).Find(&items).Error; err != nil { return code("database_unavailable") }
	if len(items) != len(q.Refs) || len(items) > 200 { return code("dependency_unavailable") }
	for _, item := range items {
		if item.ActionKind != "merge_label" || item.Owner != p.UserID { return code("dependency_unavailable") }
		if _, err = s.labelMergeFileOn(tx, p, plan.SpaceID, item.FileID, item.VersionID, false); err != nil { return err }
	}
	return nil
}

func (s *Service) CreateLabelMergePlan(ctx context.Context, p Principal, id string, q contract.LabelMergePreview, key string) (OrganizationPlanView, error) {
	if !s.Config.LabelManagement { return OrganizationPlanView{}, code("unsupported_action") }
	if err := collectionChangeKey(key); err != nil { return OrganizationPlanView{}, err }
	if !validLabelMerge(q) { return OrganizationPlanView{}, code("invalid_request") }
	body, _ := json.Marshal(struct { SpaceID string `json:"space_id"`; Request contract.LabelMergePreview `json:"request"` }{id, q})
	digest := Hash(string(body))
	var plan OrganizationPlan
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, true); err != nil { return err }
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "label.merge", key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest { return code("idempotency_conflict") }
			if err = tx.Where("id = ?", receipt.ResourceID).First(&plan).Error; err != nil { return resourceReadError(err) }
			return s.labelMergePlanAuthorizeOn(tx, p, plan, false)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) { return code("database_unavailable") }
		if err = s.labelMergeCatalogOn(tx, p, id, q); err != nil { return err }
		now := time.Now().UTC()
		request, _ := json.Marshal(q)
		actions, _ := json.Marshal([]OrganizationAction{{Kind: "merge_label", Namespace: "manual", Value: string(request)}})
		detailRequest := q
		detailRequest.Refs = nil
		detail, _ := json.Marshal(detailRequest)
		plan = OrganizationPlan{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: id, Actions: string(actions), Revision: 1, Phase: "previewed", Risk: "approval_required", ItemsTotal: int64(len(q.Refs)), Key: "label-merge:" + Hash(p.UserID+"\x00"+p.TenantID+"\x00"+key), Fingerprint: digest, CreatedAt: now, UpdatedAt: now}
		items := make([]OrganizationPlanItem, 0, len(q.Refs))
		type member struct { fileID, version string }
		members := make([]member, 0, len(q.Refs))
		seen := map[string]bool{}
		for _, ref := range q.Refs {
			fileID, version, err := s.resolveRef(ref)
			if err != nil { return err }
			if seen[fileID] { return code("invalid_request") }
			seen[fileID] = true
			members = append(members, member{fileID: fileID, version: version})
		}
		// Keep the caller's refs in the request/digest, but acquire every file
		// lock in identity order across services and overlapping previews.
		slices.SortFunc(members, func(a, b member) int { return strings.Compare(a.fileID, b.fileID) })
		for _, member := range members {
			fileID, version := member.fileID, member.version
			file, err := s.labelMergeFileOn(tx, p, id, fileID, version, true)
			if err != nil { return err }
			rows, assignments, err := labelMergeAssignmentsOn(tx, p.UserID, fileID)
			if err != nil { return err }
			hasSource := false
			for _, row := range rows { if row.Namespace == "manual" && row.Label == q.SourceKey { hasSource = true } }
			if !hasSource { return code("revision_conflict") }
			items = append(items, OrganizationPlanItem{ID: ID(), PlanID: plan.ID, ItemKey: fileID, Owner: p.UserID, FileID: fileID, VersionID: version, Path: file.Path, ActionKind: "merge_label", ActionDetail: string(detail), BeforeValue: assignments, AfterValue: q.TargetKey, Status: "pending", CreatedAt: now, UpdatedAt: now})
		}
		impact, _ := json.Marshal(struct { Request json.RawMessage `json:"request"`; Items []OrganizationPlanItem `json:"items"` }{body, items})
		plan.ImpactDigest = Hash(string(impact))
		if err = tx.Create(&plan).Error; err != nil { return code("database_unavailable") }
		if err = tx.Create(&items).Error; err != nil { return code("database_unavailable") }
		if err = tx.Create(&WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "label.merge", RequestKey: key, RequestDigest: digest, ResourceID: plan.ID, ResultRevision: 1, CreatedAt: now}).Error; err != nil { return code("database_unavailable") }
		return nil
	})
	if err != nil { return OrganizationPlanView{}, err }
	return s.OrganizationPlanViewFor(p, plan, "", 50)
}

func (s *Service) LookupLabelMergePlan(ctx context.Context, p Principal, id, key string) (OrganizationPlanView, error) {
	if err := collectionChangeKey(key); err != nil { return OrganizationPlanView{}, err }
	var plan OrganizationPlan
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, false); err != nil { return err }
		var receipt WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "label.merge", key).First(&receipt).Error; err != nil { return resourceReadError(err) }
		if err := tx.Where("id = ? AND space_id = ?", receipt.ResourceID, id).First(&plan).Error; err != nil { return resourceReadError(err) }
		return s.labelMergePlanAuthorizeOn(tx, p, plan, false)
	})
	if err != nil { return OrganizationPlanView{}, err }
	return s.OrganizationPlanViewFor(p, plan, "", 50)
}

// The existing organization execution loop calls this once per member. The
// assignment and terminal outcome commit together, so unknown retries never
// apply a merge twice or report an uncommitted outcome.
func (s *Service) executeLabelMergeItem(p Principal, plan OrganizationPlan, item OrganizationPlanItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var current OrganizationPlanItem
		if err := tx.Where("id = ? AND plan_id = ?", item.ID, plan.ID).First(&current).Error; err != nil { return resourceReadError(err) }
		if current.Status != "pending" { return nil }
		q, err := labelMergeRequest(plan)
		if err != nil { return err }
		if err = s.propertySpaceOn(tx, p, plan.SpaceID, true); err != nil { return err }
		if !s.Config.LabelManagement { return code("unsupported_action") }
		// Fence concurrent catalog changes before inspecting the frozen revisions.
		locked := tx.Model(&WorkspaceLabelCatalog{}).Where("space_id = ? AND namespace = ? AND revision = ?", plan.SpaceID, "manual", q.ExpectedRevision).Update("revision", q.ExpectedRevision)
		if locked.Error != nil { return code("database_unavailable") }
		failure := error(nil)
		if locked.RowsAffected != 1 { failure = code("revision_conflict") }
		if failure == nil { failure = s.labelMergeCatalogOn(tx, p, plan.SpaceID, q) }
		if failure == nil {
			var file File
			file, failure = s.labelMergeFileOn(tx, p, plan.SpaceID, item.FileID, item.VersionID, true)
			if failure == nil && file.Path != item.Path { failure = code("plan_stale") }
		}
		var rows []FileLabelAssignment
		var digest string
		if failure == nil { rows, digest, failure = labelMergeAssignmentsOn(tx, p.UserID, item.FileID) }
		if failure != nil && safeCode(failure) == "database_unavailable" { return failure }
		if failure == nil && digest != current.BeforeValue { failure = code("revision_conflict") }
		status, errorCode := "done", ""
		if failure != nil { status, errorCode = "stale", safeCode(failure) } else {
			target := false
			for _, row := range rows { if row.Namespace == "manual" && row.Label == q.TargetKey { target = true } }
			now := time.Now().UTC()
			if !target {
				if err = tx.Create(&FileLabelAssignment{ID: ID(), Owner: p.UserID, FileID: item.FileID, Namespace: "manual", Label: q.TargetKey, AuthorKind: "human", ActorID: p.UserID, CreatedAt: now, UpdatedAt: now}).Error; err != nil { return code("database_unavailable") }
			}
			deleted := tx.Where("owner = ? AND file_id = ? AND namespace = ? AND label = ?", p.UserID, item.FileID, "manual", q.SourceKey).Delete(&FileLabelAssignment{})
			if deleted.Error != nil { return code("database_unavailable") }
			if deleted.RowsAffected == 0 { return code("revision_conflict") }
			if target {
				updated := tx.Model(&FileLabelAssignment{}).Where("owner = ? AND file_id = ? AND namespace = ? AND label = ?", p.UserID, item.FileID, "manual", q.TargetKey).Update("updated_at", now)
				if updated.Error != nil { return code("database_unavailable") }
				if updated.RowsAffected != 1 { return code("revision_conflict") }
			}
		}
		resultVersion := ""
		if status == "done" { resultVersion = item.VersionID }
		result := tx.Model(&OrganizationPlanItem{}).Where("id = ? AND status = ?", item.ID, "pending").Updates(map[string]any{"status": status, "error": errorCode, "result_version": resultVersion, "updated_at": time.Now().UTC()})
		if result.Error != nil { return code("database_unavailable") }
		if result.RowsAffected != 1 { return code("plan_stale") }
		return nil
	})
}
