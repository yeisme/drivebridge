package sdk

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// ShareReceiverClient uses the existing JSON client and its HTTP transport.
// Supply a human session's cookie jar and CSRF token for owner management;
// nil creates an anonymous receiver jar. No bearer/Agent credential is minted.
type ShareReceiverClient struct{ *Client }

type shareReceiverTransport struct {
	base   http.RoundTripper
	origin string
	csrf   string
}

func (t shareReceiverTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r := request.Clone(request.Context())
	if r.Header.Get("Authorization") == "Bearer " {
		r.Header.Del("Authorization")
	}
	r.Header.Set("Origin", t.origin)
	if t.csrf != "" {
		r.Header.Set("X-CSRF-Token", t.csrf)
	}
	return t.base.RoundTrip(r)
}

func NewShareReceiverClient(base string, jar http.CookieJar, csrf string) (*ShareReceiverClient, error) {
	c, err := New(base, "")
	if err != nil {
		return nil, err
	}
	if jar == nil {
		jar, err = cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
	}
	c.http.Jar = jar
	c.http.Transport = shareReceiverTransport{base: http.DefaultTransport, origin: c.base, csrf: csrf}
	return &ShareReceiverClient{Client: c}, nil
}

type ShareChallenge struct {
	Nonce   string    `json:"nonce"`
	Expires time.Time `json:"expires_at"`
}

type ShareOpenResult struct {
	Opened       bool   `json:"opened"`
	SessionKind  string `json:"session_kind"`
	ReceiverKind string `json:"receiver_kind"`
}

type ShareReceiverDevice struct {
	ID          string    `json:"id"`
	Fingerprint string    `json:"fingerprint"`
	Label       string    `json:"label"`
	Created     time.Time `json:"created_at"`
	LastSeen    time.Time `json:"last_seen"`
	Opens       int64     `json:"opens"`
	Revoked     bool      `json:"revoked"`
}

type ShareReceivers struct {
	Devices       []ShareReceiverDevice `json:"devices"`
	OpensUsed     int64                 `json:"opens_used"`
	OpensMax      *int64                `json:"opens_max"`
	DownloadsUsed int64                 `json:"downloads_used"`
	DownloadsMax  *int64                `json:"downloads_max"`
}

func (c *ShareReceiverClient) IssueShareChallenge(ctx context.Context, id string) (ShareChallenge, error) {
	var out ShareChallenge
	err := c.JSON(ctx, "GET", "/api/v1/shared/"+url.PathEscape(id)+"/challenge", "", nil, &out)
	return out, err
}

// The link secret only enters this local HMAC, never an HTTP body or cookie.
func (c *ShareReceiverClient) OpenShareProof(ctx context.Context, id, nonce, secret, passcode string) (ShareOpenResult, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id + ":" + nonce))
	body := map[string]string{"nonce": nonce, "proof": hex.EncodeToString(mac.Sum(nil)), "passcode": passcode}
	var out ShareOpenResult
	err := c.JSON(ctx, "POST", "/api/v1/shared/"+url.PathEscape(id)+"/proof-open", "", body, &out)
	return out, err
}

func (c *ShareReceiverClient) RegisterShareReceiverDevice(ctx context.Context, id string, public ed25519.PublicKey, label string) (ShareReceiverDevice, error) {
	if len(public) != ed25519.PublicKeySize {
		return ShareReceiverDevice{}, errors.New("invalid_share")
	}
	body := map[string]string{"public_key": hex.EncodeToString(public), "label": label}
	var out ShareReceiverDevice
	err := c.JSON(ctx, "POST", "/api/v1/shared/"+url.PathEscape(id)+"/devices", "", body, &out)
	return out, err
}

func (c *ShareReceiverClient) OpenShareWithDevice(ctx context.Context, id, device, nonce string, private ed25519.PrivateKey) (ShareOpenResult, error) {
	if len(private) != ed25519.PrivateKeySize {
		return ShareOpenResult{}, errors.New("invalid_share")
	}
	body := map[string]string{"device_id": device, "nonce": nonce, "signature": hex.EncodeToString(ed25519.Sign(private, []byte(id+":"+nonce)))}
	var out ShareOpenResult
	err := c.JSON(ctx, "POST", "/api/v1/shared/"+url.PathEscape(id)+"/device-open", "", body, &out)
	return out, err
}

func (c *ShareReceiverClient) ListShareReceivers(ctx context.Context, id string) (ShareReceivers, error) {
	var out ShareReceivers
	err := c.JSON(ctx, "GET", "/api/v1/shares/"+url.PathEscape(id)+"/receivers", "", nil, &out)
	return out, err
}

func (c *ShareReceiverClient) RevokeShareReceiver(ctx context.Context, id, device string) error {
	return c.JSON(ctx, "DELETE", "/api/v1/shares/"+url.PathEscape(id)+"/receivers/"+url.PathEscape(device), "", nil, nil)
}
