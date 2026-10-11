package service

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// Controlled automation: authenticated change polling, human-managed
// rules, idempotent intents and pause semantics. Events carry safe
// references only; consumers re-authorize entity reads themselves.

// ChangeEventView is one delivered change fact.
type ChangeEventView struct {
	EventID          string `json:"event_id"`
	ResourceRef      string `json:"resource_ref"`
	ResourceRevision int64  `json:"resource_revision"`
	Kind             string `json:"kind"`
	Origin           string `json:"origin"`
	Sequence         int64  `json:"sequence"`
}

// ChangesView is one bounded delivery page.
type ChangesView struct {
	Events     []ChangeEventView `json:"events"`
	NextCursor string            `json:"next_cursor,omitempty"`
	Coverage   string            `json:"coverage"`
}

// changesCursorPayload is the opaque delivery position: owner-bound and
// sequence-anchored. It is a delivery position, never a permission token.
type changesCursorPayload struct {
	Owner    string `json:"owner"`
	SpaceID  string `json:"space_id"`
	Sequence int64  `json:"sequence"`
}

func (s *Service) sealChangesCursor(owner, spaceID string, sequence int64) (string, error) {
	body, _ := json.Marshal(changesCursorPayload{Owner: owner, SpaceID: spaceID, Sequence: sequence})
	nonce := make([]byte, s.crypt.NonceSize())
	crand.Read(nonce)
	sealed := s.crypt.Seal(nonce, nonce, body, []byte("changes-cursor"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Service) openChangesCursor(cursor, owner, spaceID string) (int64, error) {
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if e != nil || len(raw) < s.crypt.NonceSize() {
		return 0, code("cursor_expired")
	}
	plain, e := s.crypt.Open(nil, raw[:s.crypt.NonceSize()], raw[s.crypt.NonceSize():], []byte("changes-cursor"))
	if e != nil {
		return 0, code("cursor_expired")
	}
	var payload changesCursorPayload
	if json.Unmarshal(plain, &payload) != nil || payload.Owner != owner || payload.SpaceID != spaceID {
		return 0, code("cursor_expired")
	}
	return payload.Sequence, nil
}

// ReadChanges delivers one bounded page of committed file events inside
// the caller's authorized scope. Delivery is at-least-once; consumers
// deduplicate by event_id. An expired cursor means a controlled snapshot
// rebuild — never a silent gap jump. The cursor binds the caller's owner
// and the canonical query so it cannot be replayed across scopes.
func (s *Service) ReadChanges(p Principal, spaceID, cursor string, limit int) (ChangesView, error) {
	limit = orgPageLimit(limit)
	if spaceID != "" {
		if _, e := s.space(p, spaceID); e != nil {
			return ChangesView{}, e
		}
		// Agents authorize under their own bound prefix: an empty path
		// never matches a prefixed grant, which used to refuse the
		// agent's own space outright.
		prefix := ""
		if p.Agent != nil {
			prefix = p.Agent.Prefix
		}
		if e := s.allow(p, spaceID, prefix, "read"); e != nil {
			return ChangesView{}, e
		}
	} else if p.Agent != nil {
		// An omitted space never widens an agent to the owner-wide
		// event stream: the agent must name its authorized space.
		return ChangesView{}, code("forbidden")
	}
	sequence := int64(0)
	if cursor != "" {
		var e error
		sequence, e = s.openChangesCursor(cursor, p.dataOwner(), spaceID)
		if e != nil {
			return ChangesView{}, e
		}
	}
	// Events are owner-scoped facts from the transactional outbox; the
	// space filter narrows through the file's space.
	tx := s.DB.Model(&OutboxEvent{}).Where("owner_scope = ? AND source_sequence > ?", p.dataOwner(), sequence).Order("source_sequence")
	var events []OutboxEvent
	if e := tx.Limit(limit + 1).Find(&events).Error; e != nil {
		return ChangesView{}, code("database_unavailable")
	}
	more := len(events) > limit
	if more {
		events = events[:limit]
	}
	view := ChangesView{Events: []ChangeEventView{}, Coverage: "complete"}
	if more {
		view.Coverage = "partial"
	}
	for _, event := range events {
		kind := "file_committed"
		if event.Tombstone {
			kind = "file_purged"
		}
		var f File
		if spaceID != "" {
			if s.DB.First(&f, "id = ?", event.ResourceID).Error != nil || f.SpaceID != spaceID {
				continue
			}
			// The delivered stream stays inside the agent's prefix: the
			// intersection of the space filter and the prefix grant.
			if p.Agent != nil && p.Agent.Prefix != "" && !contains(p.Agent.Prefix, f.Path) {
				continue
			}
		}
		// Safe references only: no names, bodies or temporary URLs ride
		// the event; consumers re-authorize the entity read themselves.
		view.Events = append(view.Events, ChangeEventView{EventID: event.EventID, ResourceRef: "drivebridge://" + s.Instance + "/file/" + event.ResourceID, ResourceRevision: event.ResourceRevision, Kind: kind, Origin: "upload", Sequence: event.SourceSequence})
	}
	if more && len(events) > 0 {
		next, e := s.sealChangesCursor(p.dataOwner(), spaceID, events[len(events)-1].SourceSequence)
		if e != nil {
			return ChangesView{}, e
		}
		view.NextCursor = next
	}
	return view, nil
}

// CreateChangeSubscriptionRequest opens one polling consumer.
type CreateChangeSubscriptionRequest struct {
	ConsumerID string `json:"consumer_id"`
	SpaceID    string `json:"space_id"`
}

// CreateChangeSubscription registers one consumer inside the caller's
// authorized scope; subscriptions never enable whole-space automation by
// themselves.
func (s *Service) CreateChangeSubscription(p Principal, q CreateChangeSubscriptionRequest) (ChangeSubscription, error) {
	if q.ConsumerID == "" || len(q.ConsumerID) > 200 {
		return ChangeSubscription{}, code("invalid_request")
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return ChangeSubscription{}, e
	}
	if e := s.allow(p, q.SpaceID, "", "read"); e != nil {
		return ChangeSubscription{}, e
	}
	now := time.Now().UTC()
	sub := ChangeSubscription{ID: ID(), Owner: p.dataOwner(), ConsumerID: q.ConsumerID, SpaceID: q.SpaceID, CreatedAt: now, UpdatedAt: now}
	if e := s.DB.Create(&sub).Error; e != nil {
		return ChangeSubscription{}, code("database_unavailable")
	}
	return sub, nil
}

// AckChangesRequest advances the subscription checkpoint.
type AckChangesRequest struct {
	Checkpoint int64 `json:"checkpoint"`
}

// AckChanges confirms delivery handling up to a delivered sequence. The
// checkpoint is monotonic and may never jump past the highest delivered
// sequence: acking ahead is refused, not silently clamped.
func (s *Service) AckChanges(p Principal, subscriptionID string, q AckChangesRequest) (ChangeSubscription, error) {
	var sub ChangeSubscription
	if s.DB.First(&sub, "id = ? AND owner = ?", subscriptionID, p.dataOwner()).Error != nil {
		return ChangeSubscription{}, code("not_found")
	}
	if q.Checkpoint < sub.Checkpoint {
		return ChangeSubscription{}, code("invalid_request")
	}
	var max int64
	s.DB.Model(&OutboxEvent{}).Where("owner_scope = ?", p.dataOwner()).Select("COALESCE(MAX(source_sequence),0)").Scan(&max)
	if q.Checkpoint > max {
		return ChangeSubscription{}, code("ack_ahead")
	}
	if e := s.DB.Model(&ChangeSubscription{}).Where("id = ?", sub.ID).Updates(map[string]any{"checkpoint": q.Checkpoint, "updated_at": time.Now().UTC()}).Error; e != nil {
		return ChangeSubscription{}, code("database_unavailable")
	}
	s.DB.First(&sub, "id = ?", sub.ID)
	return sub, nil
}
