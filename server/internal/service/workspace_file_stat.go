package service

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

// Metadata uses the current fixed version's connection and the same narrow
// authorization as content, without acquiring a storage URL or reading bytes.
func (s *Service) workspaceFileStat(p Principal, id string) (map[string]any, error) {
	return s.workspaceFileVersionStat(p, id, "")
}

func (s *Service) workspaceFileVersionStat(p Principal, id, versionID string) (map[string]any, error) {
	pinned := versionID != ""
	var file File
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		if versionID == "" {
			versionID = file.VersionID
		}
		current, version, err := s.workspaceReadVersionOn(tx, p, id, versionID)
		if err != nil {
			return err
		}
		file = current
		if pinned {
			file.VersionID, file.Size, file.MIME, file.Updated = version.ID, version.Size, version.MIME, version.Created
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"file": file, "ref": s.FileRef(file)}, nil
}

func (s *Service) workspaceFileGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	view, err := s.workspaceFileStat(p, r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}
