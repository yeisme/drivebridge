package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type aigoraAccess struct {
	mu                 sync.Mutex
	base, key, context string
	expires            time.Time
	http               *http.Client
}

// NewAigora exchanges the user's key only at the configured Aigora origin.
// Copies returned by Scoped share a safe token cache, never mutable scope.
func NewAigora(base, aigoraOrigin, key string) (*Client, error) {
	client, err := New(base, "")
	if err != nil {
		return nil, err
	}
	gateway, err := New(aigoraOrigin, "")
	if err != nil || key == "" || len(key) > 8192 {
		return nil, errors.New("invalid_aigora_configuration")
	}
	client.access = &aigoraAccess{base: gateway.base, key: key, http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	return client, nil
}

func (a *aigoraAccess) credential(ctx context.Context, method string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if a.context != "" && now.Add(10*time.Second).Before(a.expires) {
		return a.context, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/v1/service-access/exchange", bytes.NewBufferString(`{"audience":"drivebridge"}`))
	if err != nil {
		return "", errors.New("aigora_unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+a.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.http.Do(req)
	if err != nil {
		if (method == http.MethodGet || method == http.MethodHead) && a.context != "" && time.Now().Before(a.expires) {
			return a.context, nil
		}
		return "", errors.New("aigora_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode >= 500 && (method == http.MethodGet || method == http.MethodHead) && a.context != "" && time.Now().Before(a.expires) {
			return a.context, nil
		}
		return "", errors.New("aigora_access_denied")
	}
	var response struct {
		Data struct {
			Context string `json:"principal_context"`
			Expires int64  `json:"expires_at"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 32768)).Decode(&response) != nil || strings.Count(response.Data.Context, ".") != 2 || response.Data.Context == a.key || response.Data.Expires <= time.Now().Unix() || response.Data.Expires > time.Now().Add(5*time.Minute).Unix() {
		return "", errors.New("invalid_service_context")
	}
	a.context = response.Data.Context
	a.expires = time.Unix(response.Data.Expires, 0)
	return a.context, nil
}
