package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// newSyncCommand is the manual, human-triggered bidirectional sync surface.
// It is deliberately not an MCP tool: agents never trigger syncs.
func newSyncCommand(c *cli) *cobra.Command {
	var resync, dryRun bool
	var maxDelete int
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Guarded bidirectional sync between the space's local root and its remote root (manual only).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			// The journal lives with the private state, independent of the
			// user's cache directory.
			report, err := s.Sync(cmd.Context(), space, bridge.SyncOptions{Resync: resync, DryRun: dryRun, MaxDelete: maxDelete, Workdir: filepath.Join(c.effective.Values["state"], "bisync")})
			if err != nil {
				if report.OperationID != "" || report.Status == "refused" || report.Status == "planned" {
					// Refusals and aborted plans still emit their plan for
					// review before the error surfaces.
					_ = c.mode.write("sync", "refused", err.Error(), report)
				}
				return err
			}
			summary := fmt.Sprintf("Sync %s: %s.", report.OperationID, report.Status)
			if report.DryRun {
				summary = fmt.Sprintf("Dry run: %d addition(s), %d update(s), %d deletion(s), %d conflict(s).", len(report.Additions1)+len(report.Additions2), len(report.Updates1)+len(report.Updates2), len(report.Deletions1)+len(report.Deletions2), len(report.Conflicts))
			} else if report.Resync {
				summary = fmt.Sprintf("Baseline resync %s completed.", report.OperationID)
			}
			return c.mode.write("sync", "success", summary, report)
		},
	}
	cmd.Flags().String("space", "", "Space identifier")
	cmd.Flags().BoolVar(&resync, "resync", false, "Establish the first baseline (or repair an untrusted journal); path1 (local) seeds new paths")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report the four-class plan without changing either side")
	cmd.Flags().IntVar(&maxDelete, "max-delete", 0, "Deletion cap in percent per side (default 25; raise explicitly to override a safety abort)")
	return cmd
}
