package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

const savedQueryBudget = 64 * 1024

func savedQueryActorOn(tx *gorm.DB, p Principal) error {
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	var user User
	if err := tx.Where("id = ?", p.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("forbidden")
		}
		return code("database_unavailable")
	}
	if user.Disabled || user.Archived {
		return code("forbidden")
	}
	return nil
}

func (s *Service) savedQuerySpacesOn(tx *gorm.DB, p Principal, q FileQueryRequest) error {
	if err := validateFileQuery(q); err != nil {
		return err
	}
	if len(q.SpaceIDs) > 100 {
		return code("query_budget_exceeded")
	}
	seen := map[string]bool{}
	for _, id := range q.SpaceIDs {
		if id == "" || len(id) > 512 || seen[id] {
			return code("invalid_query")
		}
		seen[id] = true
		if p.ScopeSpace != "" && p.ScopeSpace != id {
			return code("not_found")
		}
		var space Space
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.dataOwner(), p.TenantID).First(&space).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		// A saved query contains arbitrary predicates and a human name. Unlike
		// result rows, these cannot be safely projected to an Agent path prefix.
		// Require root metadata access, never infer it from a query's filters.
		if err := s.allowOn(tx, p, id, "", "read"); err != nil {
			return err
		}
	}
	return nil
}

func decodeSavedQuery(raw string) (FileQueryRequest, error) {
	var q FileQueryRequest
	if len(raw) > savedQueryBudget {
		return q, code("query_budget_exceeded")
	}
	if !utf8.ValidString(raw) || strings.TrimSpace(raw) == "null" {
		return q, code("invalid_query")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&q); err != nil {
		return q, code("invalid_query")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return q, code("invalid_query")
	}
	return q, nil
}
