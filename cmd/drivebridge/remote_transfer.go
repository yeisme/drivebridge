package main

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

func routeTransfer(c *cli, cmd *cobra.Command) {
	local := cmd.RunE
	for _, entry := range [][2]string{{"remote-url", "Remote service origin"}, {"remote-token-file", "Private scoped credential file"}, {"local-root", "Allowed absolute local directory"}} {
		cmd.Flags().String(entry[0], "", entry[1])
	}
	cmd.RunE = func(cmd *cobra.Command, argv []string) error {
		endpoint := c.effective.Values["remote_url"]
		if endpoint == "" {
			return local(cmd, argv)
		}
		name := cmd.Name()
		if name == "mkdir" || name == "changes" || name == "purge" {
			return fmt.Errorf("remote_unsupported")
		}
		a, err := remoteclient.New(endpoint, c.effective.Values["remote_token_file"], c.effective.Values["local_root"])
		if err != nil {
			return err
		}
		defer a.Close()
		get := func(k string) string { v, _ := cmd.Flags().GetString(k); return v }
		args := map[string]any{"space_id": get("space"), "ref": get("ref"), "path": get("path"), "query": get("query"), "mime_type": get("mime-type"), "after": get("after")}
		action := name
		switch name {
		case "upload":
			file := get("file")
			if filepath.IsAbs(file) {
				file, err = filepath.Rel(c.effective.Values["local_root"], file)
				if err != nil {
					return fmt.Errorf("path_not_allowed")
				}
			}
			name := get("name")
			if name == "" {
				name = filepath.Base(file)
			}
			if name == "." || name == ".." || path.Base(name) != name || filepath.Base(name) != name {
				return fmt.Errorf("invalid_upload")
			}
			dir := get("dir")
			if dir != "" && (!filepath.IsLocal(dir) || path.Clean(dir) != dir) {
				return fmt.Errorf("path_not_allowed")
			}
			key := get("idempotency-key")
			if key == "" {
				key = "cli-" + stableKey(get("space")+"\x00"+get("file")+"\x00"+get("name"))
			}
			args["local_path"], args["path"], args["idempotency_key"] = file, path.Join(dir, name), key
			args["caller"], args["task_ref"] = get("caller"), get("task-ref")
		case "download":
			file := get("out")
			if filepath.IsAbs(file) {
				file, err = filepath.Rel(c.effective.Values["local_root"], file)
				if err != nil {
					return fmt.Errorf("path_not_allowed")
				}
			}
			args["local_path"] = file
		case "ls":
			action = "list"
		case "find":
			action = "search"
		case "resume", "cancel":
			args["operation_id"] = get("id")
		case "operation":
			action = "operation_get"
			args["operation_id"] = get("id")
		}
		out, err := a.Call(cmd.Context(), action, args)
		if err != nil {
			return err
		}
		if name == "upload" {
			out = awaitRemoteTerminal(cmd.Context(), a, out)
		}
		if name == "ls" || name == "find" {
			if list, ok := out.([]any); ok {
				for _, item := range list {
					if f, ok := item.(map[string]any); ok && !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\n", f["path"])
					}
				}
				out = map[string]any{"count": len(list), "files": list}
			}
		}
		status, message := "success", "Remote file operation completed."
		if name == "upload" {
			if receipt, ok := out.(remoteclient.UploadResult); ok {
				// The CLI is synchronous for the user, mirroring the local
				// awaitTerminal contract: poll operation_get until the
				// remote operation settles instead of returning an
				// in-flight receipt from a dying process.
				if receipt.Status == "completed" {
					return c.mode.write(name, "success", fmt.Sprintf("Uploaded via %s.", receipt.ID), out)
				}
				return c.mode.fail(name, terminalCodeValues(receipt.Error, receipt.Status), fmt.Sprintf("Remote operation %s stopped: %s (%d/%d bytes).", receipt.ID, receipt.Status, receipt.Transferred, receipt.Size), map[string]any{"operation": receipt})
			}
		}
		if name == "resume" {
			if op, ok := out.(map[string]any); ok && op["status"] == "awaiting_upload" {
				status, message = "awaiting_upload", "Original operation is waiting for bytes; upload the same source with the original idempotency key."
			}
		}
		return c.mode.write(name, status, message, out)
	}
}

// awaitRemoteTerminal polls operation_get until the remote upload reaches a
// terminal state (or awaits bytes only the caller can re-send). Non-upload
// results and missing IDs pass through untouched.
func awaitRemoteTerminal(ctx context.Context, a *remoteclient.Adapter, out any) any {
	receipt, ok := out.(remoteclient.UploadResult)
	if !ok || receipt.ID == "" {
		return out
	}
	for {
		switch receipt.Status {
		case "completed", "failed", "cancelled", "interrupted", "awaiting_upload":
			return receipt
		}
		select {
		case <-ctx.Done():
			return receipt
		case <-time.After(200 * time.Millisecond):
		}
		state, err := a.Client.Operation(ctx, receipt.ID)
		if err != nil {
			return receipt
		}
		receipt.Operation = state.Operation
	}
}
