package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ServiceAdmission is the local ledger of a hosted business-operation
// reservation at the Aigora service-billing authority. One row belongs to
// exactly one upload Operation; recovery reconciliation replays unreported
// rows after restarts.
type ServiceAdmission struct {
	ID            string `gorm:"primaryKey"` // equals the local Operation ID
	TenantID      string `gorm:"index"`
	UserID        string `gorm:"index"`
	EntitlementID string
	OperationRef  string `gorm:"uniqueIndex"`
	Units         int64
	Status        string // reserved | reported
	Outcome       string // delivered | failed | unknown_accept
	ActualUnits   int64
	Created       time.Time
	Reported      *time.Time
}

const admissionService = "drivebridge"

// admissionBound actions are handled at the business operation boundary
// (createUpload), never per HTTP request: the boundary either reserves usage
// with the authority or rejects the write.
func admissionBound(action string) bool {
	return action == "uploads.create" || action == "mcp.create_upload"
}

// admissionContinuation actions move bytes of an already-admitted operation
// (or release its reservation); they never open a new chargeable unit and stay
// online-checked by the guard.
func admissionContinuation(action string) bool {
	return action == "uploads.part" || action == "uploads.complete" || action == "operations.cancel" || action == "mcp.complete_upload" || action == "mcp.cancel"
}

func (s *Service) admissionHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// reserveServiceAdmission is called by the upload boundary before any local
// state changes. Legacy principals are never billed; hosted writes without a
// configured reporter credential or entitlement stay fail-closed.
func (s *Service) reserveServiceAdmission(p Principal, operationID string, units int64, entitlementID string) (*ServiceAdmission, error) {
	if p.ServiceIdentity == nil {
		return nil, nil
	}
	if s.Config.ServiceAdmissionCredential == "" || entitlementID == "" || units <= 0 || operationID == "" {
		return nil, code("service_admission_required")
	}
	body, _ := json.Marshal(map[string]any{"entitlement_id": entitlementID, "operation_id": operationID, "units": units, "principal_context": p.ServiceIdentity.token})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, strings.TrimRight(s.serviceIdentityEndpoint(), "/")+"/internal/v1/service-billing/"+admissionService+"/reserve", bytes.NewReader(body))
	if err != nil {
		return nil, code("identity_unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Config.ServiceAdmissionCredential)
	res, err := s.admissionHTTPClient().Do(req)
	if err != nil {
		return nil, code("identity_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		var response struct {
			Data struct {
				OperationRef  string `json:"operation_ref"`
				Status        string `json:"status"`
				ReservedUnits int64  `json:"reserved_units"`
			} `json:"data"`
		}
		if json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&response) != nil || response.Data.OperationRef == "" || response.Data.Status != "reserved" || response.Data.ReservedUnits != units {
			return nil, code("identity_unavailable")
		}
		return &ServiceAdmission{ID: operationID, TenantID: p.TenantID, UserID: p.UserID, EntitlementID: entitlementID, OperationRef: response.Data.OperationRef, Units: units, Status: "reserved", Created: time.Now().UTC()}, nil
	}
	switch admissionRemoteCode(res) {
	case "quota_exceeded":
		return nil, code("quota_exceeded")
	case "insufficient_funds":
		return nil, code("insufficient_funds")
	case "confirmation_required":
		return nil, code("service_admission_required")
	case "operation_conflict":
		return nil, code("idempotency_conflict")
	case "identity_unavailable":
		return nil, code("identity_unavailable")
	}
	return nil, code("service_admission_required")
}

func admissionRemoteCode(res *http.Response) string {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&envelope)
	return envelope.Error.Code
}

// reportServiceAdmission settles one reservation after its local operation
// reached a terminal state. Reporting is best-effort: an unreachable authority
// never fails the local operation; reconciliation retries later.
func (s *Service) reportServiceAdmission(op Operation, outcome string, actual int64) error {
	var row ServiceAdmission
	if s.DB.Where("id = ? AND status = ?", op.ID, "reserved").First(&row).Error != nil {
		return nil
	}
	return s.reportAdmissionRef(row, outcome, actual)
}

// reportAdmissionRef settles a reservation whose local row may not exist:
// when the transaction that would persist it failed after the authority had
// already reserved, only the in-memory reference can ever report it.
func (s *Service) reportAdmissionRef(row ServiceAdmission, outcome string, actual int64) error {
	body, _ := json.Marshal(map[string]any{"operation_ref": row.OperationRef, "outcome": outcome, "actual_units": actual})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, strings.TrimRight(s.serviceIdentityEndpoint(), "/")+"/internal/v1/service-billing/"+admissionService+"/report", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Config.ServiceAdmissionCredential)
	res, err := s.admissionHTTPClient().Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 16384))
	if res.StatusCode != 200 {
		return nil
	}
	now := time.Now().UTC()
	if e := s.DB.Model(&ServiceAdmission{}).Where("id = ? AND status = ?", row.ID, "reserved").Updates(map[string]any{"status": "reported", "outcome": outcome, "actual_units": actual, "reported": &now}).Error; e != nil {
		return nil
	}
	_ = s.event(s.DB, Principal{UserID: row.UserID, TenantID: row.TenantID}, Space{}, "admission.report", row.OperationRef, row.ID, "success", outcome)
	return nil
}

// reconcileServiceAdmissions replays unreported reservations after restart.
// A reservation whose local operation finished settles to the same outcome a
// live report would have sent; a missing operation is reported as accepted
// with unknown usage so the authority holds it for manual settlement instead
// of silently releasing or charging it.
func (s *Service) reconcileServiceAdmissions() error {
	if s.Config.ServiceAdmissionCredential == "" {
		return nil
	}
	var rows []ServiceAdmission
	if e := s.DB.Where("status = ?", "reserved").Limit(100).Find(&rows).Error; e != nil {
		return e
	}
	for _, row := range rows {
		var op Operation
		err := s.DB.First(&op, "id = ?", row.ID).Error
		switch {
		case err == nil && op.Status == "completed":
			_ = s.reportServiceAdmission(op, "delivered", op.Size)
		case err == nil && op.Status == "cancelled":
			_ = s.reportServiceAdmission(op, "failed", 0)
		case err == gorm.ErrRecordNotFound:
			_ = s.reportUnknownAdmission(row)
		}
		// Locally failed operations stay recoverable, so their reservation is
		// held at the authority instead of being released or settled here.
	}
	return nil
}

func (s *Service) reportUnknownAdmission(row ServiceAdmission) error {
	body, _ := json.Marshal(map[string]any{"operation_ref": row.OperationRef, "outcome": "unknown_accept", "actual_units": 0})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, strings.TrimRight(s.serviceIdentityEndpoint(), "/")+"/internal/v1/service-billing/"+admissionService+"/report", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Config.ServiceAdmissionCredential)
	res, err := s.admissionHTTPClient().Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 16384))
	if res.StatusCode != 200 {
		return nil
	}
	now := time.Now().UTC()
	if e := s.DB.Model(&ServiceAdmission{}).Where("id = ? AND status = ?", row.ID, "reserved").Updates(map[string]any{"status": "reported", "outcome": "unknown_accept", "actual_units": 0, "reported": &now}).Error; e != nil {
		return nil
	}
	_ = s.event(s.DB, Principal{UserID: row.UserID, TenantID: row.TenantID}, Space{}, "admission.reconcile", row.OperationRef, row.ID, "success", "unknown_accept")
	return nil
}
