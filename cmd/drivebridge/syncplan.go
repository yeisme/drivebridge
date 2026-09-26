package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// newSyncPlanCommand freezes a reviewed plan: the four-class preview plus
// conflicts, persisted for explicit apply. Planning never writes either side.
func newSyncPlanCommand(c *cli) *cobra.Command {
	var resync bool
	var maxDelete int
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Freeze a reviewed sync plan (scan both sides, persist the item set; no writes yet).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			result, err := s.PlanSync(cmd.Context(), space, bridge.PlanSyncOptions{Resync: resync, MaxDelete: maxDelete, Workdir: filepath.Join(c.effective.Values["state"], "bisync")})
			if err != nil {
				if result.Plan.ID != "" || result.RefusalReason != "" || result.Coverage != nil {
					// Refusals still emit the coverage or ratio for review.
					code, message := codify(err)
					if result.RefusalReason != "" {
						message = result.RefusalReason
					}
					_ = c.mode.fail("sync.plan", code, message, result)
					return errQuiet
				}
				return err
			}
			deletions := countClasses(result.Items, "delete_local") + countClasses(result.Items, "delete_remote")
			summary := fmt.Sprintf("Plan %s frozen: phase %s, %d item(s), %d deletion(s), %d conflict(s); apply with drivebridge sync apply %s --revision %d.",
				result.Plan.ID, result.Plan.Phase, result.TotalItems, deletions, result.Plan.Conflicts, result.Plan.ID, result.Plan.Revision)
			return c.mode.write("sync.plan", "success", summary, result)
		},
	}
	cmd.Flags().String("space", "", "Space identifier")
	cmd.Flags().BoolVar(&resync, "resync", false, "Plan the first baseline; same-name differences are previewed as conflicts, never hidden")
	cmd.Flags().IntVar(&maxDelete, "max-delete", 0, "Deletion cap in percent per side (default 25; raise explicitly to override)")
	return cmd
}

func countClasses(items []bridge.SyncItem, class string) int {
	n := 0
	for _, item := range items {
		if item.Class == class {
			n++
		}
	}
	return n
}

// newSyncApplyCommand executes a frozen plan from the user terminal after
// revalidating writer, roots, baseline and both sides. Agents never apply.
func newSyncApplyCommand(c *cli) *cobra.Command {
	var revision int64
	cmd := &cobra.Command{
		Use:   "apply PLAN_ID",
		Short: "Apply a frozen sync plan after revalidation (user terminal only; agents have no apply surface).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if revision <= 0 {
				return fail("version_conflict", "Apply requires the plan revision you reviewed: drivebridge sync status PLAN_ID shows it.")
			}
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			result, err := s.ApplySyncPlan(cmd.Context(), args[0], bridge.ApplySyncPlanOptions{ExpectedRevision: revision})
			if err != nil {
				code, message := codify(err)
				if hint, ok := result.Recovery[code]; ok && hint != "" {
					message = hint
				} else if result.RefusalReason != "" {
					message = result.RefusalReason
				}
				_ = c.mode.fail("sync.apply", code, message, result)
				return errQuiet
			}
			summary := fmt.Sprintf("Plan %s applied: %s (operation %s).", result.Plan.ID, result.Plan.Phase, result.Plan.OperationID)
			return c.mode.write("sync.apply", "success", summary, result)
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "Plan revision you reviewed (shown by sync status; stale revisions are refused)")
	return cmd
}

// newSyncStatusCommand reads a frozen plan with one keyset item page.
func newSyncStatusCommand(c *cli) *cobra.Command {
	var after uint64
	var limit int
	cmd := &cobra.Command{
		Use:   "status PLAN_ID",
		Short: "Read a frozen sync plan, one item page and recovery hints.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			result, err := s.SyncPlanGet(args[0], after, limit)
			if err != nil {
				code, message := codify(err)
				if hint, ok := result.Recovery[code]; ok && hint != "" {
					message = hint
				}
				_ = c.mode.fail("sync.status", code, message, result)
				return errQuiet
			}
			summary := fmt.Sprintf("Plan %s: phase %s, %d item(s) total, page shows %d.", result.Plan.ID, result.Plan.Phase, result.TotalItems, len(result.Items))
			return c.mode.write("sync.status", "success", summary, result)
		},
	}
	cmd.Flags().Uint64Var(&after, "after", 0, "Keyset cursor from next_after")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size (default 50, max 200)")
	return cmd
}

// newRetentionCommand groups read-only cleanup classification surfaces.
func newRetentionCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retention",
		Short: "Read-only retention classification for a space (preview only; nothing is deleted).",
	}
	preview := &cobra.Command{
		Use:   "preview",
		Short: "Classify staging, local trash, protected VFS bytes and remote references; never deletes or purges.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			report, err := s.RetentionPreview(cmd.Context(), space)
			if err != nil {
				return err
			}
			summary := "Cleanup preview: "
			for i, class := range report.Classes {
				if i > 0 {
					summary += ", "
				}
				summary += class.Class + " " + strconv.Itoa(class.Files) + " file(s)"
				if class.Protected {
					summary += " (protected)"
				}
			}
			summary += "."
			return c.mode.write("retention.preview", "success", summary, report)
		},
	}
	preview.Flags().String("space", "", "Space identifier")
	cmd.AddCommand(preview)
	return cmd
}
