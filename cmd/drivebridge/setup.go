package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newSetupCommand(c *cli) *cobra.Command {
	cmd := spaceAddCommand(c, "setup")
	cmd.Short = "Connect an existing storage folder without copying internal IDs."
	cmd.Flags().Set("space", "personal")
	cmd.Flags().Set("max-bytes", "1073741824")
	if cwd, err := os.Getwd(); err == nil {
		cmd.Flags().Lookup("local-root").Value.Set(cwd)
	}
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if c.effective.Values["remote_url"] != "" {
			return fmt.Errorf("local_setup_only")
		}
		if err := c.installRclone(); err != nil {
			return err
		}
		remote, _ := cmd.Flags().GetString("remote")
		if remote == "" {
			names := rcloneconfig.LoadedData().GetSectionList()
			if len(names) == 1 {
				remote = names[0]
			} else {
				info, err := os.Stdin.Stat()
				if err != nil || info.Mode()&os.ModeCharDevice == 0 {
					return fmt.Errorf("remote_selection_required")
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Choose a configured remote (use 'drivebridge auth' to connect one):", strings.Join(names, ", "))
				remote, err = bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil {
					return fmt.Errorf("remote_selection_required")
				}
				remote = strings.TrimSpace(remote)
			}
			if err := cmd.Flags().Set("remote", remote); err != nil {
				return err
			}
		}
		kind := rcloneconfig.GetValue(remote, "type")
		if kind == "" {
			return fmt.Errorf("authentication_required: run drivebridge auth")
		}
		if kind != "onedrive" {
			cmd.Flags().Set("backend", "generic")
		}
		pathFlag := "root-path"
		if kind != "onedrive" {
			pathFlag = "remote-path"
		}
		if !cmd.Flags().Changed(pathFlag) {
			info, err := os.Stdin.Stat()
			if err != nil || info.Mode()&os.ModeCharDevice == 0 {
				return fmt.Errorf("folder_path_required")
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Folder path inside the remote (existing folder; no internal ID):")
			folder, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil || strings.TrimSpace(folder) == "" {
				return fmt.Errorf("folder_path_required")
			}
			cmd.Flags().Set(pathFlag, strings.TrimSpace(folder))
		}
		local, _ := cmd.Flags().GetString("local-root")
		if configured := c.effective.Values["local_root"]; configured != "" {
			local = configured
		}
		abs, err := filepath.Abs(local)
		if err != nil {
			return fmt.Errorf("local_root_unavailable")
		}
		cmd.Flags().Set("local-root", abs)
		return run(cmd, args)
	}
	return cmd
}

func discoverSpaceRoot(ctx context.Context, remote, folder, drive, root string) (string, string, error) {
	if folder == "" {
		return drive, root, nil
	}
	if root != "" {
		return "", "", fmt.Errorf("root_selection_conflict")
	}
	discovered, id, err := bridge.NewOneDrive().DiscoverRoot(ctx, remote, folder)
	if err != nil {
		return "", "", err
	}
	if drive != "" && drive != discovered {
		return "", "", fmt.Errorf("space_not_allowed")
	}
	return discovered, id, nil
}
