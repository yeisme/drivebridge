package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newRecoveryCommand(c *cli, action string) *cobra.Command {
	cmd := &cobra.Command{Use: action + " [operation-id]", Short: action + " the original transfer; never create a generation task.", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("operation_id_required")
		}
		s, release, err := c.openService(true)
		if err != nil {
			return err
		}
		defer release()
		if base := c.effective.Values["scaena_url"]; base != "" {
			s.Output, err = bridge.NewScaenaOutput(base, scaenaTokenReader(c.effective.Values["scaena_token_file"]))
			if err != nil {
				return err
			}
		}
		var op bridge.Operation
		if action == "resume" {
			op, err = s.Resume(id)
			if err == nil {
				op = awaitTerminal(s, id)
			}
		} else {
			op, err = s.Cancel(id)
		}
		if err != nil {
			return err
		}
		status := "success"
		if op.Status == "failed" || op.Status == "interrupted" {
			status = "failed"
		}
		if err = c.mode.write(action, status, fmt.Sprintf("%s %s. %s", op.ID, op.Status, recoveryHint(op.Error)), op); err != nil {
			return err
		}
		if status == "failed" {
			return fmt.Errorf("operation_incomplete")
		}
		return nil
	}}
	cmd.Flags().String("id", "", "Original operation ID")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return cmd.Flags().Set("id", args[0])
		}
		return nil
	}
	routeTransfer(c, cmd)
	return cmd
}
