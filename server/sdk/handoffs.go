package sdk

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AllowedHandoffPurposes is the additive Server purpose allowlist. Unknown
// values are rejected as invalid_binding; reference remains valid.
var AllowedHandoffPurposes = []string{"reference", "vault", "notes-attachments", "materials", "saveback", "owner-store"}

type HandoffBinding struct {
	ID         string `json:"id"`
	AgentID    string `json:"agent_id"`
	Consumer   string `json:"consumer"`
	ProjectRef string `json:"project_ref"`
	Purpose    string `json:"purpose"`
	SpaceID    string `json:"space_id"`
	Prefix     string `json:"prefix"`
	Disabled   bool   `json:"disabled"`
}
type BindReferenceRequest struct {
	BindingID          string     `json:"binding_id"`
	Ref                string     `json:"ref"`
	Consumer           string     `json:"consumer"`
	ProjectRef         string     `json:"project_ref"`
	Purpose            string     `json:"purpose"`
	ExpectedSHA256     string     `json:"expected_sha256"`
	ConsumerRequestRef string     `json:"consumer_request_ref,omitempty"`
	RequestExpiresAt   *time.Time `json:"request_expires_at,omitempty"`
}
type Handoff struct {
	ID               string     `json:"grant_id"`
	BindingID        string     `json:"binding_id"`
	FileID           string     `json:"file_id"`
	VersionID        string     `json:"version_id"`
	Consumer         string     `json:"consumer"`
	ProjectRef       string     `json:"project_ref"`
	Purpose          string     `json:"purpose"`
	SHA256           string     `json:"sha256"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RequestExpiresAt *time.Time `json:"request_expires_at,omitempty"`
	Cancelled        bool       `json:"cancelled"`
}

func (c *Client) BindReference(ctx context.Context, q BindReferenceRequest, key string) (Handoff, error) {
	var v Handoff
	err := c.JSON(ctx, http.MethodPost, "/api/v1/handoffs", key, q, &v)
	return v, err
}
func (c *Client) GetHandoff(ctx context.Context, id string) (Handoff, error) {
	var v Handoff
	err := c.JSON(ctx, http.MethodGet, "/api/v1/handoffs/"+url.PathEscape(id), "", nil, &v)
	return v, err
}
func (c *Client) RenewHandoff(ctx context.Context, id string) (Handoff, error) {
	var v Handoff
	err := c.JSON(ctx, http.MethodPost, "/api/v1/handoffs/"+url.PathEscape(id)+"/renew", "", struct{}{}, &v)
	return v, err
}
func (c *Client) CancelHandoff(ctx context.Context, id string) (Handoff, error) {
	var v Handoff
	err := c.JSON(ctx, http.MethodPost, "/api/v1/handoffs/"+url.PathEscape(id)+"/cancel", "", struct{}{}, &v)
	return v, err
}
func (c *Client) OpenHandoff(ctx context.Context, id string) (io.ReadCloser, error) {
	resp, err := c.request(ctx, http.MethodGet, "/api/v1/handoffs/"+url.PathEscape(id)+"/content", "", nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
