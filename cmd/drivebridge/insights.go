package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/sdk"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"github.com/yeisme/drivebridge/internal/bridge"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (c *cli) readInsights() (*insights.Store, func(), error) {
	state := c.effective.Values["state"]
	info, e := os.Stat(state)
	if e != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, nil, fmt.Errorf("state_not_private")
	}
	database := filepath.Join(state, "state.sqlite")
	if _, e = os.Stat(database); e != nil {
		return nil, nil, fmt.Errorf("state_unavailable")
	}
	uri := url.URL{Scheme: "file", Path: database, RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	db, e := gorm.Open(sqlite.Open(uri.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		return nil, nil, fmt.Errorf("state_unavailable")
	}
	raw, e := db.DB()
	if e != nil {
		return nil, nil, e
	}
	return insights.ReadOnly(db), func() { raw.Close() }, nil
}
func (c *cli) queryInsights(kind string, q url.Values) (any, error) {
	if endpoint := c.effective.Values["remote_url"]; endpoint != "" {
		token, e := scaenaTokenReader(c.effective.Values["remote_token_file"])()
		if e != nil {
			return nil, e
		}
		client, e := sdk.New(endpoint, token)
		if e != nil {
			return nil, e
		}
		path := "/api/v1/usage/" + kind
		if strings.HasPrefix(kind, "audit_") {
			path = "/api/v1/audit/" + strings.TrimPrefix(kind, "audit_")
		}
		var out any
		e = client.JSON(context.Background(), "GET", path+"?"+q.Encode(), "", nil, &out)
		return out, e
	}
	store, release, e := c.readInsights()
	if e != nil {
		return nil, e
	}
	defer release()
	out, err := bridge.QueryInsights(store, kind, q)
	if err == nil && (kind == "export" || kind == "audit_export") {
		recorded := c.noteInsightExport(context.Background(), kind)
		return map[string]any{"export": out, "audit_recorded": recorded}, nil
	}
	return out, err
}
func (c *cli) requestScan(ctx context.Context, q bridge.ScanRequest) (any, error) {
	if endpoint := c.effective.Values["remote_url"]; endpoint != "" {
		token, e := scaenaTokenReader(c.effective.Values["remote_token_file"])()
		if e != nil {
			return nil, e
		}
		client, e := sdk.New(endpoint, token)
		if e != nil {
			return nil, e
		}
		var out any
		e = client.JSON(ctx, "POST", "/api/v1/usage/scans", "", q, &out)
		return out, e
	}
	socket := filepath.Join(c.effective.Values["state"], "insights.sock")
	if info, e := os.Lstat(socket); e == nil && info.Mode()&os.ModeSocket != 0 {
		tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}}
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr, Timeout: time.Hour}
		b, _ := json.Marshal(q)
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/scan", bytes.NewReader(b))
		resp, e := client.Do(req)
		if e != nil {
			return nil, fmt.Errorf("insights_owner_unavailable")
		}
		defer resp.Body.Close()
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&envelope) != nil {
			return nil, fmt.Errorf("scan_request_failed")
		}
		var out any
		e = json.Unmarshal(envelope.Data, &out)
		return out, e
	}
	s, release, e := c.openService(false)
	if e != nil {
		return nil, fmt.Errorf("insights_owner_unavailable: stop the owning MCP or enable its --insights socket")
	}
	defer release()
	return s.RunScanRequest(ctx, q)
}
func newUsageCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "usage", Short: "Inspect versioned resource usage; never infer provider billing."}
	for _, kind := range []string{"summary", "series", "breakdown", "events", "scans", "export"} {
		root.AddCommand(insightQueryCommand(c, kind, kind))
	}
	var space, resume, cancelID string
	var enable, disable, configure bool
	var maxEntries, maxSeconds int
	scan := &cobra.Command{Use: "scan", Short: "Scan an explicitly enabled storage scope with a bounded budget.", RunE: func(cmd *cobra.Command, _ []string) error {
		if enable && disable {
			return fmt.Errorf("conflicting_scan_policy")
		}
		q := bridge.ScanRequest{Space: space, Resume: resume, CancelID: cancelID, MaxEntries: maxEntries, MaxSeconds: maxSeconds, ConfigureOnly: configure}
		if enable {
			v := true
			q.Enabled = &v
		}
		if disable {
			v := false
			q.Enabled = &v
		}
		out, e := c.requestScan(cmd.Context(), q)
		return writeInsights(c, "usage.scan", out, e)
	}}
	scan.Flags().StringVar(&space, "space", "", "Existing authorized space")
	scan.Flags().StringVar(&cancelID, "cancel-id", "", "Request cancellation of an active scan in this space")
	scan.Flags().StringVar(&resume, "resume-id", "", "Resume a provider-supported scan checkpoint")
	scan.Flags().BoolVar(&enable, "enable", false, "Enable daily scans for this space")
	scan.Flags().BoolVar(&disable, "disable", false, "Disable daily scans for this space")
	scan.Flags().BoolVar(&configure, "configure-only", false, "Update policy without accessing storage")
	scan.Flags().IntVar(&maxEntries, "max-entries", 100000, "Maximum entries per run (1-10000000)")
	scan.Flags().IntVar(&maxSeconds, "max-seconds", 600, "Maximum seconds per run (1-3600)")
	root.AddCommand(scan)
	return root
}
func newAuditCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "audit", Short: "Query redacted security events; no tamper-proof guarantee."}
	root.AddCommand(insightQueryCommand(c, "list", "audit_events"), insightQueryCommand(c, "export", "audit_export"))
	return root
}
func insightQueryCommand(c *cli, name, kind string) *cobra.Command {
	cmd := &cobra.Command{Use: name, Short: "Read or export scoped usage/audit projections (UTC).", RunE: func(cmd *cobra.Command, _ []string) error {
		q := url.Values{}
		for _, pair := range [][2]string{{"space", "space_id"}, {"connection", "connection_id"}, {"from", "from"}, {"to", "to"}, {"actor", "actor"}, {"caller", "caller"}, {"direction", "direction"}, {"purpose", "purpose"}, {"group", "group"}, {"action", "action"}, {"outcome", "outcome"}, {"resource", "resource"}, {"correlation", "correlation"}, {"before", "before"}, {"limit", "limit"}} {
			v, _ := cmd.Flags().GetString(pair[0])
			if v != "" {
				q.Set(pair[1], v)
			}
		}
		out, e := c.queryInsights(kind, q)
		return writeInsights(c, strings.ReplaceAll(kind, "_", "."), out, e)
	}}
	for _, pair := range [][2]string{{"space", "Space filter"}, {"connection", "Storage connection filter"}, {"from", "First UTC date YYYY-MM-DD (default last 30 days)"}, {"to", "Last UTC date YYYY-MM-DD"}, {"actor", "Authenticated actor ID"}, {"caller", "Declared caller label"}, {"direction", "Observed transfer direction"}, {"purpose", "Transfer purpose"}, {"group", "Breakdown: space, connection, actor, caller, direction, purpose"}, {"action", "Audit action"}, {"outcome", "Audit outcome"}, {"resource", "Resource ID, never a file path"}, {"correlation", "Audit correlation ID"}, {"before", "Pagination cursor"}, {"limit", "Audit page size (1-200)"}} {
		cmd.Flags().String(pair[0], "", pair[1])
	}
	return cmd
}
func writeInsights(c *cli, command string, out any, e error) error {
	if e != nil {
		_ = c.mode.write(command, "failed", "Insight query failed; inspect readiness, scope and filters.", map[string]any{"error_code": insightError(e)})
		return errors.New(insightError(e))
	}
	status := "success"
	if run, ok := out.(insights.ScanRun); ok && run.Status != "complete" {
		status = "partial"
	}
	if m, ok := out.(map[string]any); ok {
		if v, _ := m["status"].(string); v == "partial" || v == "failed" || v == "cancelled" {
			status = "partial"
		}
	}
	summary := "Insight projection ready (UTC); missing data is not zero. Use --json for details."
	if s, ok := out.(insights.Summary); ok {
		summary = fmt.Sprintf("Effective=%d B; observed=%d B; alerts=%d; collection since %s. Observed bytes are not provider billing.", s.Totals.Effective, s.Totals.Observed, len(s.Alerts), s.Collection.Started.Format(time.RFC3339))
	}
	if s, ok := out.(insights.Summary); ok && c.mode.Agent {
		out = map[string]any{"metric_version": s.MetricVersion, "effective_bytes": s.Totals.Effective, "observed_bytes": s.Totals.Observed, "alerts": len(s.Alerts), "collection_started_at": s.Collection.Started.Format(time.RFC3339)}
	}
	return c.mode.write(command, status, summary, out)
}
func insightError(e error) string {
	if e == nil {
		return ""
	}
	v := e.Error()
	if len(v) > 80 {
		return "insights_unavailable"
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return "insights_unavailable"
		}
	}
	return v
}

func (c *cli) noteInsightExport(ctx context.Context, kind string) bool {
	socket := filepath.Join(c.effective.Values["state"], "insights.sock")
	if info, e := os.Lstat(socket); e == nil && info.Mode()&os.ModeSocket != 0 {
		tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}}
		defer tr.CloseIdleConnections()
		b, _ := json.Marshal(map[string]string{"kind": kind})
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/export", bytes.NewReader(b))
		resp, e := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Do(req)
		if e != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 204
	}
	s, release, e := c.openService(false)
	if e != nil {
		return false
	}
	defer release()
	return s.RecordInsightExport(kind) == nil
}
