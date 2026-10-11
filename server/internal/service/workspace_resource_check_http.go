package service

import (
	"context"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) LookupResourceIntegrity(ctx context.Context, p Principal, id string, revision uint64) (ResourceIntegrityReceipt, error) {
	var result WorkspaceResourceCheck
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, _, digest, err := s.resourceIntegrityTargetsOn(tx, p, id, revision)
		if err != nil {
			return err
		}
		if err := tx.Where("owner = ? AND tenant_id = ? AND descriptor_id = ? AND descriptor_revision = ? AND checker_version = ?", p.UserID, p.TenantID, id, revision, resourceIntegrityChecker).First(&result).Error; err != nil {
			return resourceReadError(err)
		}
		if result.TargetDigest != digest || result.CreatedAt.IsZero() || (result.State != "passed" && result.State != "failed") {
			return code("revision_conflict")
		}
		return nil
	})
	if err != nil {
		return ResourceIntegrityReceipt{}, err
	}
	return resourceIntegrityReceipt(result), nil
}

func (s *Service) resourceCheckHandler(lookup bool) endpoint {
	return func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		input, err := contract.DecodeRequest[contract.ResourceCheckRequest](http.MaxBytesReader(w, r.Body, 1024))
		if err != nil {
			return code("invalid_request")
		}
		if input.ExpectedRevision == 0 || input.ExpectedRevision > 9007199254740991 || input.CheckerVersion != resourceIntegrityChecker {
			return code("invalid_request")
		}
		var receipt ResourceIntegrityReceipt
		if lookup {
			receipt, err = s.LookupResourceIntegrity(r.Context(), p, r.PathValue("id"), input.ExpectedRevision)
		} else {
			receipt, err = s.CheckResourceIntegrity(r.Context(), p, r.PathValue("id"), input.ExpectedRevision)
		}
		if err != nil {
			return err
		}
		respond(w, receipt)
		return nil
	}
}
