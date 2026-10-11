package service

import "net/http"

func (s *Service) snapshotIndexHandler(w http.ResponseWriter, r *http.Request, p Principal) error {
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		return code("invalid_request")
	}
	out, err := s.ListCollectionSnapshots(r.Context(), p, r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, out)
	return nil
}
