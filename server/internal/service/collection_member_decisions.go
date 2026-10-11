package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// CollectionMemberDecisions returns every explicit decision or no edit base.
// Rule evaluation and visible member counts cannot substitute for this view.
func (s *Service) CollectionMemberDecisions(ctx context.Context, p Principal, id string) (contract.CollectionMemberDecisions, error) {
	var out contract.CollectionMemberDecisions
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !collectionHumanPrincipal(p) {
			return code("forbidden")
		}
		c, err := s.collectionReadAdmissionOn(tx, p, id)
		if err != nil {
			return err
		}
		if err = s.allowOn(tx, p, c.SpaceID, "", "write"); err != nil {
			return err
		}
		if c.Revision == 0 || c.Revision >= 9007199254740991 || (c.Kind != "manual" && c.Kind != "rule") {
			return code("dependency_unavailable")
		}
		var rows []CollectionMember
		if err = tx.Where("collection_id = ?", id).Order("file_id ASC, id ASC").Limit(10001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 10000 {
			return code("query_budget_exceeded")
		}
		out = contract.CollectionMemberDecisions{CollectionID: c.ID, Revision: c.Revision, Items: []contract.CollectionMemberDecision{}}
		seen := map[string]bool{}
		wireBytes := 0
		for _, row := range rows {
			decoded, e := hex.DecodeString(row.FileID)
			if e != nil || len(decoded) != 24 || hex.EncodeToString(decoded) != row.FileID || seen[row.FileID] || row.Owner != p.UserID || (row.Decision != "include" && row.Decision != "exclude") {
				return code("dependency_unavailable")
			}
			seen[row.FileID] = true
			var file File
			if err = tx.Where("id = ? AND owner = ?", row.FileID, p.UserID).First(&file).Error; err != nil {
				return resourceReadError(err)
			}
			if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
				return code("not_found")
			}
			if err = s.collectionReadSpaceOn(tx, p, file.SpaceID); err != nil {
				return err
			}
			if err = s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
				return err
			}
			item := contract.CollectionMemberDecision{FileID: file.ID, SpaceID: file.SpaceID, Path: file.Path, Decision: row.Decision}
			raw, e := json.Marshal(item)
			if e != nil {
				return code("dependency_unavailable")
			}
			wireBytes += len(raw) + 1
			if wireBytes > 1024*1024-2048 {
				return code("query_budget_exceeded")
			}
			out.Items = append(out.Items, item)
		}
		// Reject a mixed observation on databases whose default isolation permits
		// another transaction to update the collection while rows are being read.
		latest, err := s.collectionReadAdmissionOn(tx, p, id)
		if err != nil {
			return err
		}
		if latest.Revision != c.Revision || latest.SpaceID != c.SpaceID || latest.Kind != c.Kind || latest.Rule != c.Rule {
			return code("query_changed")
		}
		return s.allowOn(tx, p, c.SpaceID, "", "write")
	})
	if err != nil {
		return contract.CollectionMemberDecisions{}, err
	}
	return out, nil
}

func (s *Service) collectionMemberDecisionsHandler(w http.ResponseWriter, r *http.Request, p Principal) error {
	if r.URL.RawQuery != "" {
		return code("invalid_request")
	}
	out, err := s.CollectionMemberDecisions(r.Context(), p, r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, out)
	return nil
}
