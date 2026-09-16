package main

import (
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newRelayCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "relay", Short: "Transfer a pinned creative file to a configured owner input (eikona, scaena or sonora)"}
	var q bridge.RelayTarget
	set := &cobra.Command{Use: "target-set <id>", Short: "Register an immutable owner-approved relay target", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, close, err := c.openService(false)
		if err != nil {
			return err
		}
		defer close()
		q.ID = args[0]
		if err := s.ConfigureRelay(q); err != nil {
			return err
		}
		return c.mode.write("relay.target-set", "success", "Relay target registered.", q)
	}}
	set.Flags().StringVar(&q.SpaceID, "space", "", "allowed source space")
	set.Flags().StringVar(&q.Project, "project", "", "target owner project scope")
	set.Flags().StringVar(&q.ConnectionID, "connection", "", "owner-issued local-relay connection ID")
	set.Flags().StringVar(&q.Owner, "owner", "eikona", "original input owner: eikona, scaena or sonora")
	set.Flags().StringVar(&q.Origin, "url", "", "trusted owner origin (https, or http on loopback)")
	set.Flags().StringVar(&q.KeyFile, "key-file", "", "private input-only owner key file")
	for _, name := range []string{"space", "project", "connection", "url", "key-file"} {
		_ = set.MarkFlagRequired(name)
	}
	root.AddCommand(set)
	var target, key string
	start := &cobra.Command{Use: "start <file-ref>", Short: "Relay a pinned file without calling a generation provider", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, close, err := c.openService(true)
		if err != nil {
			return err
		}
		defer close()
		op, err := s.StartRelay(cmd.Context(), args[0], target, key)
		if err != nil {
			return err
		}
		op = awaitTerminal(s, op.ID)
		if op.Status != "completed" {
			return c.mode.fail("relay.start", terminalCode(op), "Relay transfer stopped. Use drivebridge operation with the original ID; restore the source or target before drivebridge resume. Never regenerate.", map[string]any{"operation": op})
		}
		return c.mode.write("relay.start", "success", "Relay operation finished; inspect its input reference and status.", op)
	}}
	start.Flags().StringVar(&target, "target", "", "registered relay target")
	start.Flags().StringVar(&key, "idempotency-key", "", "stable logical transfer key")
	_ = start.MarkFlagRequired("target")
	_ = start.MarkFlagRequired("idempotency-key")
	root.AddCommand(start)
	root.AddCommand(&cobra.Command{Use: "cancel <operation-id>", Short: "Cancel the original local and remote input operation", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, close, err := c.openService(true)
		if err != nil {
			return err
		}
		defer close()
		op, err := s.CancelRelay(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return c.mode.write("relay.cancel", "success", "Relay cancellation requested.", op)
	}})
	for _, action := range []string{"targets", "target-disable"} {
		use := action
		args := cobra.NoArgs
		if action == "target-disable" {
			use += " <id>"
			args = cobra.ExactArgs(1)
		}
		root.AddCommand(&cobra.Command{Use: use, Short: "Inspect or disable relay targets", Args: args, RunE: func(cmd *cobra.Command, args []string) error {
			s, close, err := c.openService(false)
			if err != nil {
				return err
			}
			defer close()
			if action == "target-disable" {
				if err := s.DisableRelay(args[0]); err != nil {
					return err
				}
			}
			rows, err := s.RelayTargets()
			if err != nil {
				return err
			}
			return c.mode.write("relay.targets", "success", "Configured relay targets.", rows)
		}})
	}
	return root
}
