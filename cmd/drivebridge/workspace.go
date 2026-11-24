package main

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

func newWorkspaceCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "workspace", Short: "Discover and recover compact workspace targets."}
	root.AddCommand(newWorkspaceSearchCommand(c), newWorkspaceInspectCommand(c), newWorkspaceRecoverCommand(c))
	return root
}

func workspaceRemoteCall(c *cli, action string, args map[string]any) error {
	url := c.effective.Values["remote_url"]
	if url == "" {
		return c.mode.fail("workspace."+action[strconvPrefix(action):], "remote_url_required", "The compact workspace CLI currently requires a configured remote service.", nil)
	}
	adapter, err := remoteclient.NewWithAigora(url, c.effective.Values["remote_token_file"], c.effective.Values["local_root"], c.effective.Values["aigora_url"])
	if err != nil {
		return err
	}
	defer adapter.Close()
	out, err := adapter.Call(context.Background(), action, args)
	if err != nil {
		return err
	}
	return c.mode.write("workspace."+action[strconvPrefix(action):], "success", "Workspace operation completed.", out)
}

func strconvPrefix(action string) int { return len("workspace.") }

func newWorkspaceSearchCommand(c *cli) *cobra.Command {
	var kind, query, space string
	var limit int
	cmd := &cobra.Command{Use: "search", Short: "Search compact workspace targets.", RunE: func(cmd *cobra.Command, _ []string) error {
		return workspaceRemoteCall(c, "workspace.search", map[string]any{"target_kind": kind, "query": query, "space_id": space, "limit": strconv.Itoa(limit)})
	}}
	cmd.Flags().StringVar(&kind, "kind", "file", "Target kind: file, operation, or submission")
	cmd.Flags().StringVar(&query, "query", "", "Search text")
	cmd.Flags().StringVar(&space, "space", "", "Space identifier")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results (1-50)")
	return cmd
}

func newWorkspaceInspectCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{Use: "inspect <target-ref>", Short: "Inspect one compact workspace target.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		return workspaceRemoteCall(c, "workspace.inspect", map[string]any{"target_kind": kind, "target_ref": args[0]})
	}}
	cmd.Flags().String("kind", "file", "Target kind: file, operation, or submission")
	return cmd
}

func newWorkspaceRecoverCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{Use: "recover <target-ref>", Short: "Recover the original compact workspace operation.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		kind, _ := cmd.Flags().GetString("kind")
		return workspaceRemoteCall(c, "workspace.recover", map[string]any{"target_kind": kind, "target_ref": args[0]})
	}}
	cmd.Flags().String("kind", "operation", "Target kind: operation or submission")
	return cmd
}
