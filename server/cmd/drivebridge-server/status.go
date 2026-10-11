package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"unicode"

	_ "github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
)

// openSnapshotDB opens the state database read-only for projections that
// must never disturb a running serve.
func openSnapshotDB(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro&_pragma=busy_timeout(5000)")
}

// newStatusCommand projects one deployment's health in a single envelope:
// remote mode probes a running serve (healthz + identity + spaces); local
// mode reads the private state snapshot read-only. Either way the operator
// gets exactly one recommended next step.
func newStatusCommand(c *cli) *cobra.Command {
	var url, tokenFile string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Project deployment health from a running server or the local state snapshot.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url != "" {
				return remoteStatus(c, url, tokenFile)
			}
			return localStatus(c)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Base URL of a running deployment (remote mode)")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "Bearer user-key file (0600) for remote mode")
	return cmd
}

func remoteStatus(c *cli, url, tokenFile string) error {
	if tokenFile == "" {
		return fail("token_file_required", "Remote mode needs --token-file pointing at a 0600 user-key file.")
	}
	token, err := readPrivateToken(tokenFile)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	health, err := probeJSON(client, url+"/healthz", "")
	if err != nil {
		return fail("service_unreachable", "The deployment did not answer /healthz; check the URL or start serve.")
	}
	me, err := probeJSON(client, url+"/api/v1/me", token)
	if err != nil {
		return fail("token_rejected", "The token was rejected; verify the key is live with users list or users verify.")
	}
	spaces, err := probeJSON(client, url+"/api/v1/spaces", token)
	if err != nil {
		return fail("service_request_failed", "The spaces projection failed while health and identity answered.")
	}
	identity, _, _ := digString(me, "user", "name")
	admin, _, _ := digBool(me, "admin")
	data := map[string]any{
		"mode":      "remote",
		"reachable": true,
		"health":    health,
		"identity":  map[string]any{"name": identity, "admin": admin},
		"spaces":    spaces,
	}
	rows, _ := spaces.([]any)
	return c.mode.emitData("status", "success", fmt.Sprintf("Remote deployment reachable; identity %s; %d space(s).", identity, len(rows)), data)
}

func localStatus(c *cli) error {
	state := c.effective.Values["state"]
	data := map[string]any{"mode": "local"}
	summary := ""
	next := ""
	trio := map[string]bool{
		"instance_key": fileExists(filepath.Join(state, "instance.key")),
		"instance_id":  fileExists(filepath.Join(state, "instance.id")),
		"database":     fileExists(filepath.Join(state, "metadata.sqlite")),
	}
	data["state"] = trio
	dbPath := filepath.Join(state, "metadata.sqlite")
	if !trio["database"] {
		summary = "No local state database yet; the deployment has not been initialized."
		next = "Run drivebridge-server config init and platform-connect, then serve --enabled."
	} else {
		if db, e := openSnapshotDB(dbPath); e == nil {
			defer db.Close()
			var spaces struct {
				Count     int    `json:"spaces"`
				Max       int64  `json:"max_bytes"`
				Used      int64  `json:"used_bytes"`
				Reserved  int64  `json:"reserved_bytes"`
				Platform  string `json:"platform"`
				KeysTotal int    `json:"keys_total"`
				KeysLive  int    `json:"keys_live"`
			}
			row := db.QueryRow("SELECT (SELECT COUNT(*) FROM spaces), (SELECT COALESCE(SUM(max_bytes),0) FROM spaces), (SELECT COALESCE(SUM(used),0) FROM spaces), (SELECT COALESCE(SUM(reserved),0) FROM spaces), (SELECT COALESCE(MAX(CASE WHEN id='platform' THEN status END),'') FROM connections), (SELECT COUNT(*) FROM user_keys), (SELECT COUNT(*) FROM user_keys WHERE revoked = 0 AND archived = 0 AND disabled = 0)")
			if e := row.Scan(&spaces.Count, &spaces.Max, &spaces.Used, &spaces.Reserved, &spaces.Platform, &spaces.KeysTotal, &spaces.KeysLive); e != nil {
				return fail("state_unavailable", "The state database could not be projected.")
			}
			data["spaces"] = spaces.Count
			data["quota"] = map[string]any{"max_bytes": spaces.Max, "used_bytes": spaces.Used, "reserved_bytes": spaces.Reserved}
			data["user_keys"] = map[string]any{"total": spaces.KeysTotal, "live": spaces.KeysLive}
			data["platform_storage"] = spaces.Platform
			summary = fmt.Sprintf("Local state: %d space(s), %d/%d bytes used, %d live user key(s), platform storage %q.", spaces.Count, spaces.Used+spaces.Reserved, spaces.Max, spaces.KeysLive, spaces.Platform)
			switch {
			case spaces.Platform == "":
				next = "Run drivebridge-server platform-connect to configure platform storage."
			case spaces.KeysLive == 0:
				next = "Run drivebridge-server users import to issue the first loginable key."
			default:
				next = "Run drivebridge-server doctor for the full preflight, or serve --enabled."
			}
		} else {
			return fail("state_unavailable", "The state database could not be opened read-only.")
		}
	}
	return c.mode.emitData("status", "success", summary+" Recommended next step: "+next, data)
}

func readPrivateToken(path string) (string, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", fail("credential_unavailable", "The token file must exist, be a regular file, and disallow group/other access.")
	}
	b, e := os.ReadFile(path)
	if e != nil || len(b) > 8192 {
		return "", fail("credential_unavailable", "The token file must be readable and at most 8 KB.")
	}
	token := string(b)
	for _, r := range token {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' || !unicode.IsPrint(r) {
			return "", fail("credential_unavailable", "The token file must contain one printable token line.")
		}
	}
	return token, nil
}

func probeJSON(client *http.Client, url, token string) (any, error) {
	req, e := http.NewRequest("GET", url, nil)
	if e != nil {
		return nil, e
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var envelope struct {
		Data any `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, fmt.Errorf("invalid response")
	}
	return envelope.Data, nil
}

func digString(v any, keys ...string) (string, bool, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("not an object")
	}
	for i, k := range keys {
		if i == len(keys)-1 {
			s, _ := m[k].(string)
			return s, true, nil
		}
		next, _ := m[k].(map[string]any)
		m = next
	}
	return "", false, nil
}

func digBool(v any, key string) (bool, bool, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return false, false, fmt.Errorf("not an object")
	}
	b, _ := m[key].(bool)
	return b, true, nil
}

func fileExists(path string) bool {
	info, e := os.Lstat(path)
	return e == nil && info.Mode().IsRegular()
}
