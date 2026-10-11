package main

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/sdk"
)

func newHandoffBindingCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "handoff-binding", Short: "Configure scoped creative receivers on a running service"}
	var origin, keyFile string
	root.PersistentFlags().StringVar(&origin, "url", "", "trusted service origin")
	root.PersistentFlags().StringVar(&keyFile, "token-file", "", "private owner user-key file")
	connect := func() (*sdk.Client, error) {
		token, err := readPrivateToken(keyFile)
		if err != nil {
			return nil, err
		}
		return sdk.New(origin, token)
	}
	var q sdk.HandoffBinding
	set := &cobra.Command{Use: "create", Short: "Bind an existing scoped Agent to one consumer project and directory", RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		var result sdk.HandoffBinding
		if err := client.JSON(cmd.Context(), http.MethodPost, "/api/v1/handoff-bindings", "", q, &result); err != nil {
			return err
		}
		return c.mode.emitData("handoff-binding.create", "success", "Scoped receiver binding created.", map[string]any{"binding": result})
	}}
	set.Flags().StringVar(&q.AgentID, "agent-id", "", "existing receiver Agent ID")
	set.Flags().StringVar(&q.SpaceID, "space", "", "source space")
	set.Flags().StringVar(&q.Prefix, "prefix", "", "allowed source directory")
	set.Flags().StringVar(&q.ProjectRef, "project", "", "receiver project ID")
	set.Flags().StringVar(&q.Consumer, "consumer", "eikona", "receiver owner")
	set.Flags().StringVar(&q.Purpose, "purpose", "reference", "permitted input purpose: reference, vault, notes-attachments, materials, saveback, owner-store")
	for _, name := range []string{"agent-id", "space", "project"} {
		_ = set.MarkFlagRequired(name)
	}
	root.AddCommand(set)
	root.AddCommand(&cobra.Command{Use: "list", Short: "Inspect receiver bindings", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		var rows []sdk.HandoffBinding
		if err := client.JSON(cmd.Context(), http.MethodGet, "/api/v1/handoff-bindings", "", nil, &rows); err != nil {
			return err
		}
		return c.mode.emitData("handoff-binding.list", "success", "Configured receiver bindings.", map[string]any{"bindings": rows})
	}})
	root.AddCommand(&cobra.Command{Use: "disable <id>", Short: "Disable future acquisition without deleting delivered copies", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := connect()
		if err != nil {
			return err
		}
		if err := client.JSON(cmd.Context(), http.MethodPost, "/api/v1/handoff-bindings/"+url.PathEscape(args[0])+"/disable", "", struct{}{}, nil); err != nil {
			return err
		}
		return c.mode.emitData("handoff-binding.disable", "success", "Receiver binding disabled.", map[string]any{"disabled": true})
	}})
	return root
}

func newHandoffCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "handoff", Short: "Inspect, renew or cancel an existing receiver handoff"}
	var origin, keyFile string
	root.PersistentFlags().StringVar(&origin, "url", "", "trusted DriveBridge service origin")
	root.PersistentFlags().StringVar(&keyFile, "token-file", "", "private receiver Agent key file; renewal requires the original receiver")
	for _, action := range []string{"status", "renew", "cancel"} {
		root.AddCommand(&cobra.Command{Use: action + " <grant-id>", Short: "Use the original handoff without recreating input or granting permissions", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			token, err := readPrivateToken(keyFile)
			if err != nil {
				return err
			}
			client, err := sdk.New(origin, token)
			if err != nil {
				return err
			}
			var result sdk.Handoff
			switch action {
			case "status":
				result, err = client.GetHandoff(cmd.Context(), args[0])
			case "renew":
				result, err = client.RenewHandoff(cmd.Context(), args[0])
			case "cancel":
				result, err = client.CancelHandoff(cmd.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return c.mode.emitData("handoff."+action, "success", "Original handoff inspected. Check cancellation and expiry; imported copies are retained.", map[string]any{"handoff": result})
		}})
	}
	return root
}
