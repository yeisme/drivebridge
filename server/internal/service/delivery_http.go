package service

import "net/http"

// deliveryRoutes registers the fixed delivery surface. Publishing is the
// human authorization; receipts come from share-side callers.
func (s *Service) deliveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/shares/{id}/delivery-manifests", s.wrap("delivery.prepare", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q PrepareFileDeliveryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.PrepareFileDelivery(p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.ManifestID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/shares/{id}/delivery-manifests/{revision}", s.wrap("delivery.get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var revision uint64
		for _, c := range r.PathValue("revision") {
			if c < '0' || c > '9' {
				return code("invalid_request")
			}
			revision = revision*10 + uint64(c-'0')
		}
		view, e := s.GetFileDelivery(p, r.PathValue("id"), revision)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/shares/{id}/delivery-manifests/{revision}/publish", s.wrap("delivery.publish", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var revision uint64
		for _, c := range r.PathValue("revision") {
			if c < '0' || c > '9' {
				return code("invalid_request")
			}
			revision = revision*10 + uint64(c-'0')
		}
		view, e := s.PublishFileDelivery(p, r.PathValue("id"), revision)
		if e != nil {
			return e
		}
		markAuditTarget(r, view.ManifestID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/delivery-receipts", s.wrap("delivery.receipt", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q DeliveryReceiptRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		shareID := r.URL.Query().Get("share_id")
		if shareID == "" {
			return code("invalid_request")
		}
		receipt, e := s.RecordDeliveryReceipt(p, shareID, q)
		if e != nil {
			return e
		}
		respond(w, receipt)
		return nil
	}))
	_ = http.StatusOK
}
