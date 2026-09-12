package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// newPinCommand raises the retention marker on a reference: gc never
// collects a pinned reference regardless of age.
func newPinCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pin",
		Short: "Mark a reference as retained; gc never collects pinned references.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			f, err := s.Pin(ref)
			if err != nil {
				return err
			}
			return c.mode.write("pin", "success", fmt.Sprintf("Pinned %s.", f.Ref), map[string]any{"ref": f.Ref, "space_id": f.SpaceID, "pinned": true})
		},
	}
	cmd.Flags().String("ref", "", "Stable file reference")
	return cmd
}

// newGCCommand manually collects retention-expired references. Collection is
// a soft delete (trash) inside the space; a dry run reports the plan only.
func newGCCommand(c *cli) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Collect retention-expired references into the space trash (manual, never automatic).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			report, err := s.GC(cmd.Context(), space, dryRun)
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("Scanned %d; %d candidate(s); %d collected; %d pinned kept.", report.Scanned, len(report.Candidates), len(report.Collected), report.Pinned)
			if dryRun {
				summary = fmt.Sprintf("Dry run: scanned %d; %d candidate(s); %d pinned kept.", report.Scanned, len(report.Candidates), report.Pinned)
			}
			return c.mode.write("gc", "success", summary, report)
		},
	}
	cmd.Flags().String("space", "", "Space identifier")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report the collection plan without trashing anything")
	return cmd
}

// newLinkCommand registers an explicit cross-space reference: the target
// space gains a queryable reference to an existing remote object without
// copying bytes and without modifying the source.
func newLinkCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Register a reference to an existing object in another space (no byte copy).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			source, _ := cmd.Flags().GetString("link-ref")
			space, _ := cmd.Flags().GetString("space")
			dir, _ := cmd.Flags().GetString("dir")
			name, _ := cmd.Flags().GetString("name")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			f, err := s.LinkRef(context.Background(), source, space, dir, name)
			if err != nil {
				return err
			}
			return c.mode.write("link", "success", fmt.Sprintf("Linked %s.", f.Ref), map[string]any{"ref": f.Ref, "source_ref": source, "space_id": f.SpaceID, "name": f.Name})
		},
	}
	cmd.Flags().String("link-ref", "", "Stable reference of the existing object")
	cmd.Flags().String("space", "", "Target space identifier (same drive as the source)")
	cmd.Flags().String("dir", "", "Target directory inside the space (default root)")
	cmd.Flags().String("name", "", "Registered name (default: source name)")
	return cmd
}

// newSpaceTTLCommand configures the retention window for a space. Zero means
// permanent; gc refuses nothing on permanent spaces because nothing expires.
func newSpaceTTLCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ttl",
		Short: "Set the retention window for a space (days; 0 = permanent).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			days, _ := cmd.Flags().GetInt("days")
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			if err = s.SetSpaceTTL(space, days); err != nil {
				return err
			}
			return c.mode.write("space.ttl", "success", fmt.Sprintf("Retention for %s set to %d day(s).", space, days), map[string]any{"space_id": space, "ttl_days": days})
		},
	}
	cmd.Flags().String("space", "", "Space identifier")
	cmd.Flags().Int("days", 0, "Retention window in days (0 = permanent)")
	return cmd
}
