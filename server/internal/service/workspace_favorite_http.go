package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func workspaceMetadataKey(r *http.Request) (string, error) {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		return "", code("invalid_request")
	}
	if err := collectionChangeKey(keys[0]); err != nil {
		return "", err
	}
	return keys[0], nil
}

func (s *Service) favoriteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/favorites", s.wrapObserved("favorites.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		values, page, err := workspaceMetadataQuery(r.URL.RawQuery, "space_id")
		if err != nil {
			return err
		}
		out, err := s.WorkspaceFavorites(r.Context(), p, values.Get("space_id"), page)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/favorites", s.wrap("favorites.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.FavoriteCreate](http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			return code("invalid_request")
		}
		key, err := workspaceMetadataKey(r)
		if err != nil {
			return err
		}
		out, err := s.CreateWorkspaceFavorite(r.Context(), p, q.SpaceID, q, key)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("DELETE /api/v1/favorites/{id}", s.wrap("favorites.delete", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.FavoriteDelete](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		key, err := workspaceMetadataKey(r)
		if err != nil {
			return err
		}
		out, err := s.DeleteWorkspaceFavorite(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/favorites/lookup", s.wrapObserved("favorites.lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.FavoriteLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupWorkspaceFavorite(r.Context(), p, q.SpaceID, q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
}
