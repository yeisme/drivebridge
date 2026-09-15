package main

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newRecentCommand(c *cli) *cobra.Command {
	var space string
	var limit int
	var pending bool
	cmd := &cobra.Command{Use: "recent", Short: "Show recent transfers, provenance and recovery actions.", RunE: func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 200 {
			return fmt.Errorf("invalid_limit")
		}
		if endpoint := c.effective.Values["remote_url"]; endpoint != "" {
			return runRemoteCall(c, endpoint, c.effective.Values["remote_token_file"], c.effective.Values["local_root"], "operations", map[string]any{"space_id": space, "pending": pending, "limit": limit})
		}
		readOnly := false
		ops, err := func() ([]bridge.Operation, error) {
			s, release, err := c.openService(false)
			if err != nil {
				// Inspection never dies on the exclusive lock: fall back to
				// the read-only snapshot a registered MCP client leaves
				// perfectly readable.
				var f *cliFailure
				if errors.As(err, &f) && f.code == "state_locked" {
					snap, e := openSnapshot(c.effective.Values["state"])
					if e != nil {
						return nil, err
					}
					defer snap.close()
					readOnly = true
					return snap.operations(space, pending, limit)
				}
				return nil, err
			}
			defer release()
			q := s.DB.Order("updated DESC").Limit(limit)
			if space != "" {
				if _, err := s.Space(space); err != nil {
					return nil, err
				}
				q = q.Where("space_id = ?", space)
			}
			if pending {
				q = q.Where("status NOT IN ?", []string{"completed", "cancelled"})
			}
			out := []bridge.Operation{}
			if e := q.Find(&out).Error; e != nil {
				return nil, fmt.Errorf("state_unavailable")
			}
			return out, nil
		}()
		if err != nil {
			return err
		}
		if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
			for _, op := range ops {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s  %s\n", op.ID, op.Status, op.Caller, op.TaskRef, op.FileRef)
				if op.Error != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "  "+recoveryHint(op.Error))
				}
			}
		}
		summary := fmt.Sprintf("%d recent operation(s).", len(ops))
		if readOnly {
			summary = fmt.Sprintf("%d recent operation(s) (read-only snapshot; an MCP client holds the state).", len(ops))
		}
		data := map[string]any{"operations": ops, "count": len(ops)}
		if readOnly {
			data["read_only"] = true
		}
		return c.mode.write("recent", "success", summary, data)
	}}
	cmd.Flags().StringVar(&space, "space", "", "Restrict to one space")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum operations (1-200)")
	cmd.Flags().BoolVar(&pending, "pending", false, "Only operations that need attention")
	return cmd
}
func recoveryHint(code string) string {
	switch code {
	case "authentication_required":
		return "Reconnect using drivebridge auth; then resume the original operation."
	case "source_changed", "content_mismatch":
		return "Use the original source for recovery; changed content needs a new idempotency key."
	case "file_version_changed":
		return "List the folder again and select a current reference; the pinned version was changed remotely."
	case "target_conflict":
		return "Choose a different output path; the existing file was preserved."
	case "resume_required", "connection_lost", "graph_unreachable", "throttled":
		return "Inspect the original operation, restore connectivity, then resume it without regenerating content."
	case "quota_exceeded":
		return "Free space or adjust the storage quota before resuming the original operation."
	default:
		return "Inspect the original operation and its error before retrying; existing files are preserved."
	}
}
