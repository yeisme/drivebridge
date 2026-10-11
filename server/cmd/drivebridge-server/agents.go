package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newAgentsCommand is the operator agent-token surface on private local
// state: key-only deployments get tokens without a session+CSRF HTTP dance.
// The plaintext token appears exactly once, in this terminal or in a 0600
// file via --out.
func newAgentsCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "agents", Short: "Operator agent-token lifecycle on private local state."}

	var name, space, prefix, scopes, connection, out string
	var days int
	create := &cobra.Command{
		Use:   "create",
		Short: "Issue one agent token (one-shot plaintext, --out for a 0600 file).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			a, secret, err := s.CreateAgentOperator(space, name, prefix, scopes, connection, days)
			if err != nil {
				return err
			}
			if out != "" {
				if err := os.WriteFile(out, []byte(secret+"\n"), 0600); err != nil {
					return fail("token_write_failed", "The token file could not be written; check the path and permissions.")
				}
			}
			human := !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain)
			if human {
				fmt.Fprintf(cmd.OutOrStdout(), "agent %s created (space=%s scopes=%s expires=%s)\n", a.ID, a.SpaceID, a.Scopes, a.Expires.Format("2006-01-02"))
				if out == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "token (appears once, store it now): "+secret)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "token written once to %s (0600)\n", out)
				}
			}
			data := map[string]any{"agent": a}
			if out == "" {
				data["token"] = secret
			} else {
				data["token_file"] = out
			}
			return c.mode.emitData("agents.create", "success", fmt.Sprintf("Agent %s created; plaintext token appears exactly once.", a.ID), data)
		},
	}
	create.Flags().StringVar(&name, "name", "", "Agent name (1-120 chars)")
	create.Flags().StringVar(&space, "space", "", "Target space id (owner is derived from it)")
	create.Flags().StringVar(&prefix, "prefix", "", "Directory prefix restriction (optional)")
	create.Flags().StringVar(&scopes, "scopes", "", "Comma list: read, write, delete, import, export (default read)")
	create.Flags().StringVar(&connection, "connection", "", "Restrict import/export to one connection id (optional)")
	create.Flags().IntVar(&days, "days", 0, "Lifetime in days (1-30, default 7)")
	create.Flags().StringVar(&out, "out", "", "Write the one-shot token to this path (0600)")
	root.AddCommand(create)

	list := &cobra.Command{
		Use:   "list",
		Short: "List agent lifecycle metadata; no hashes, no tokens.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state := c.effective.Values["state"]
			dbPath := filepath.Join(state, "metadata.sqlite")
			if !fileExists(dbPath) {
				return fail("state_unavailable", "The state database does not exist yet; create an agent first.")
			}
			db, err := openSnapshotDB(dbPath)
			if err != nil {
				return fail("state_unavailable", "The state database could not be opened read-only.")
			}
			defer db.Close()
			rows, err := db.Query("SELECT id, name, space_id, prefix, scopes, expires, revoked FROM agents ORDER BY expires")
			if err != nil {
				return fail("state_unavailable", "The agents table could not be read.")
			}
			defer rows.Close()
			type agentRow struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				SpaceID string `json:"space_id"`
				Prefix  string `json:"prefix"`
				Scopes  string `json:"scopes"`
				Expires string `json:"expires_at"`
				Revoked bool   `json:"revoked"`
			}
			out := []agentRow{}
			for rows.Next() {
				var r agentRow
				if err := rows.Scan(&r.ID, &r.Name, &r.SpaceID, &r.Prefix, &r.Scopes, &r.Expires, &r.Revoked); err != nil {
					return fail("state_unavailable", "The agents table could not be read.")
				}
				out = append(out, r)
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				for _, r := range out {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s  %s  revoked=%v\n", r.ID, r.Name, r.SpaceID, r.Scopes, r.Expires, r.Revoked)
				}
			}
			return c.mode.emitData("agents.list", "success", fmt.Sprintf("%d agent(s).", len(out)), map[string]any{"agents": out, "count": len(out)})
		},
	}
	root.AddCommand(list)
	return root
}
