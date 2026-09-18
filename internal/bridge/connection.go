package bridge

import (
	"errors"
	"net/url"
	"regexp"
	"time"

	"gorm.io/gorm"
)

// Connection is a named DriveBridge connection record owned by the CLI. It is
// the consumer-facing identity (DSH and others resolve bindings by
// connection_id); credentials themselves never live on this record — only the
// credential source type and an optional private token-file reference.
type Connection struct {
	ID               string         `json:"connection_id" gorm:"primaryKey"`
	Label            string         `json:"label"`
	BaseURL          string         `json:"base_url"`
	InstanceID       string         `json:"instance_id"`
	SpaceID          string         `json:"space_id,omitempty"`
	ProjectRef       string         `json:"project_ref,omitempty"`
	CredentialSource string         `json:"credential_source"`
	TokenFile        string         `json:"-"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

var connectionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Credential sources: keychain is a declared type with no implementation yet.
var credentialSources = map[string]bool{"local-config": true, "env": true, "token-file": true, "keychain": true}

func (s *Service) validateConnection(cn Connection) error {
	if !connectionIDPattern.MatchString(cn.ID) || cn.Label == "" || cn.InstanceID == "" {
		return errors.New("invalid_connection")
	}
	u, err := url.Parse(cn.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("invalid_connection")
	}
	if !credentialSources[cn.CredentialSource] {
		return errors.New("invalid_connection")
	}
	if cn.CredentialSource == "keychain" {
		return errors.New("credential_source_unimplemented")
	}
	if cn.CredentialSource == "token-file" && cn.TokenFile == "" {
		return errors.New("invalid_connection")
	}
	if cn.SpaceID != "" {
		if _, err := s.Space(cn.SpaceID); err != nil {
			return errors.New("invalid_connection")
		}
	}
	return nil
}

func (s *Service) AddConnection(cn Connection) error {
	if err := s.validateConnection(cn); err != nil {
		return err
	}
	if err := s.DB.Create(&cn).Error; err != nil {
		return errors.New("connection_conflict")
	}
	return nil
}

// SetConnection applies a partial update; only non-empty fields change.
func (s *Service) SetConnection(id string, patch Connection) error {
	var existing Connection
	if s.DB.First(&existing, "id = ?", id).Error != nil {
		return errors.New("connection_not_found")
	}
	if patch.Label != "" {
		existing.Label = patch.Label
	}
	if patch.BaseURL != "" {
		existing.BaseURL = patch.BaseURL
	}
	if patch.InstanceID != "" {
		existing.InstanceID = patch.InstanceID
	}
	if patch.SpaceID != "" {
		existing.SpaceID = patch.SpaceID
	}
	if patch.ProjectRef != "" {
		existing.ProjectRef = patch.ProjectRef
	}
	if patch.CredentialSource != "" {
		existing.CredentialSource = patch.CredentialSource
	}
	if patch.TokenFile != "" {
		existing.TokenFile = patch.TokenFile
	}
	if err := s.validateConnection(existing); err != nil {
		return err
	}
	if err := s.DB.Save(&existing).Error; err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// RemoveConnection soft-deletes; referenced spaces and files are untouched.
func (s *Service) RemoveConnection(id string) error {
	var existing Connection
	if s.DB.First(&existing, "id = ?", id).Error != nil {
		return errors.New("connection_not_found")
	}
	if err := s.DB.Delete(&existing).Error; err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

func (s *Service) Connections() []Connection {
	var out []Connection
	s.DB.Order("id").Find(&out)
	return out
}

func (s *Service) Connection(id string) (Connection, error) {
	var cn Connection
	if s.DB.First(&cn, "id = ?", id).Error != nil {
		return cn, errors.New("connection_not_found")
	}
	return cn, nil
}
