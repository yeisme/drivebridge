package service

import (
	"net/http"
	"time"
)

// Optional authentication must never turn an invalid presented credential into
// an anonymous caller; otherwise Agent/device restrictions could be bypassed.
func (s *Service) optionalSharePrincipal(r *http.Request) (Principal, error) {
	_, cookieErr := r.Cookie(s.cookieName("db_session"))
	if r.Header.Get("Authorization") == "" && cookieErr != nil {
		return Principal{}, nil
	}
	// Receiver writes enforce Origin without requiring an owner's CSRF
	// token on the shared page; read the authenticated identity only here.
	identityRequest := r.Clone(r.Context())
	identityRequest.Method = http.MethodGet
	p, err := s.authenticate(identityRequest)
	if err != nil {
		return Principal{}, err
	}
	return s.bindContext(p, r)
}

func (s *Service) respondShareOpen(w http.ResponseWriter, id string, out ShareOpenResult) {
	age := int(time.Until(out.Expires).Seconds())
	if age < 1 {
		age = 1
	}
	http.SetCookie(w, &http.Cookie{Name: shareCookie(id), Value: out.Token, Path: "/api/v1/shared/" + id, HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteStrictMode, MaxAge: age, Expires: out.Expires})
	w.Header().Set("Cache-Control", "no-store")
	respond(w, out)
}

func (s *Service) shareReceiverEndpoint(handler endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.receiverEnabled(); err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		origin, site := r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site")
		allowed := origin == s.Config.Origin
		if r.Method == http.MethodGet && origin == "" {
			// Browsers omit Origin for same-origin GET fetches. Fetch Metadata
			// provides that provenance without relaxing mutation Origin checks.
			allowed = site == "same-origin"
		}
		if site != "" && site != "same-origin" {
			allowed = false
		}
		if !allowed {
			writeError(w, code("csrf_rejected"))
			return
		}
		p, err := s.optionalSharePrincipal(r)
		if err != nil {
			writeError(w, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err = handler(w, r, p); err != nil {
			writeError(w, err)
		}
	}
}

func (s *Service) shareReceiverRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/shared/{id}/challenge", s.shareReceiverEndpoint(func(w http.ResponseWriter, r *http.Request, _ Principal) error {
		out, err := s.IssueShareChallenge(r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shared/{id}/proof-open", s.shareReceiverEndpoint(func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ShareProofRequest
		if decode(r, &q) != nil {
			return code("invalid_share")
		}
		out, err := s.OpenShareProof(p, r.PathValue("id"), q)
		if err != nil {
			return err
		}
		s.respondShareOpen(w, r.PathValue("id"), out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shared/{id}/devices", s.shareReceiverEndpoint(func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ShareDeviceRequest
		if decode(r, &q) != nil {
			return code("invalid_share")
		}
		cookie, err := r.Cookie(shareCookie(r.PathValue("id")))
		if err != nil {
			return code("share_unavailable")
		}
		out, err := s.RegisterShareReceiverDevice(p, r.PathValue("id"), cookie.Value, q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shared/{id}/device-open", s.shareReceiverEndpoint(func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ShareDeviceOpenRequest
		if decode(r, &q) != nil {
			return code("invalid_share")
		}
		out, err := s.OpenShareWithDevice(p, r.PathValue("id"), q)
		if err != nil {
			return err
		}
		s.respondShareOpen(w, r.PathValue("id"), out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/shares/{id}/receivers", s.wrap("shares.receivers.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		out, err := s.ListShareReceivers(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("DELETE /api/v1/shares/{id}/receivers/{device}", s.wrap("shares.receivers.revoke", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if err := s.RevokeShareReceiver(p, r.PathValue("id"), r.PathValue("device")); err != nil {
			return err
		}
		respond(w, map[string]bool{"revoked": true})
		return nil
	}))
}
