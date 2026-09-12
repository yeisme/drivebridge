package main

import (
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
		s, release, err := c.openService(false)
		if err != nil {
			return err
		}
		defer release()
		q := s.DB.Order("updated DESC").Limit(limit)
		if space != "" {
			if _, err := s.Space(space); err != nil {
				return err
			}
			q = q.Where("space_id = ?", space)
		}
		if pending {
			q = q.Where("status NOT IN ?", []string{"completed", "cancelled"})
		}
		ops := []bridge.Operation{}
		if q.Find(&ops).Error != nil {
			return fmt.Errorf("state_unavailable")
		}
		if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
			for _, op := range ops {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s  %s\n", op.ID, op.Status, op.Caller, op.TaskRef, op.FileRef)
				if op.Error != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "  "+recoveryHint(op.Error))
				}
			}
		}
		return c.mode.write("recent", "success", fmt.Sprintf("%d recent operation(s).", len(ops)), map[string]any{"operations": ops, "count": len(ops)})
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
