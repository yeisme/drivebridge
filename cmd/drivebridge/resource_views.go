package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

type resourceInputError struct{ code string }

func (e resourceInputError) Error() string { return e.code }

func resourceScope(cmd *cobra.Command, next string) string {
	for _, name := range []string{"state", "config"} {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			next += " --" + name + " '" + strings.ReplaceAll(f.Value.String(), "'", "'\"'\"'") + "'"
		}
	}
	return next
}
func (c *cli) resourceDirectory(ctx context.Context, kind string) ([]map[string]any, error) {
	state := c.effective.Values["state"]
	rows := []map[string]any{}
	info, err := os.Stat(filepath.Join(state, "state.sqlite"))
	if err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("catalog_unavailable")
		}
		db, e := openReadonlyModel(state)
		if e != nil {
			return nil, e
		}
		if raw, e := db.DB(); e == nil {
			defer raw.Close()
		}
		rows, err = bridge.ResourceDirectory(ctx, db, kind)
	} else if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if kind == "connection" && len(rows) == 0 && c.effective.Values["remote_url"] != "" {
		rows = append(rows, map[string]any{"id": "default", "connection_id": "default", "label": "Default (global remote_url)", "base_url": c.effective.Values["remote_url"], "instance_id": "local", "synthetic": true})
	}
	return rows, nil
}

func resourceRead(c *cli, kind, verb string) *cobra.Command {
	cmd := &cobra.Command{Use: verb + " <" + kind + "-id>", Short: "Read " + kind + " metadata", Hidden: verb == "show"}
	if verb == "list" {
		cmd.Use = "list"
		cmd.Flags().Bool("all", false, "Show every matching row")
	}
	if kind == "connection" {
		cmd.Flags().String("id", "", "Compatibility resource id")
		_ = cmd.Flags().MarkHidden("id")
	}
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ch := make(chan []string, 1)
		ctx, cancel := context.WithTimeout(cmd.Context(), 250*time.Millisecond)
		defer cancel()
		go func() {
			local := *c
			_, effective, err := c.resolve(cmd)
			if err != nil {
				ch <- nil
				return
			}
			local.effective = effective
			rows, err := local.resourceDirectory(ctx, kind)
			ids := []string{}
			if err == nil {
				for _, r := range rows {
					id := r["id"].(string)
					if strings.HasPrefix(id, prefix) {
						ids = append(ids, id)
						if len(ids) == 50 {
							break
						}
					}
				}
			}
			ch <- ids
		}()
		select {
		case ids := <-ch:
			return ids, cobra.ShellCompDirectiveNoFileComp
		case <-ctx.Done():
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		next := resourceScope(cmd, "drivebridge "+kind+" list")
		failure := func(code, message string) error {
			env := map[string]any{"spec_version": "1.0", "mode": c.mode.name(), "command": driveCommand(kind + "." + verb), "status": "failed", "error": map[string]string{"code": code, "field": kind + "-id", "message": message, "suggestion": next}, "actions": []map[string]string{{"name": "discover", "command": next}}}
			if c.mode.JSON || c.mode.Events {
				_ = json.NewEncoder(cmd.OutOrStdout()).Encode(env)
			} else if c.mode.Agent {
				fmt.Fprintf(cmd.OutOrStdout(), "spec_version=1.0\nmode=agent\ncommand=%s\nstatus=failed\nerror.code=%s\nerror.field=%s-id\naction.discover=%s\n", env["command"], code, kind, next)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\nNext: %s\n", message, next)
			}
			return resourceInputError{code}
		}
		id := ""
		if len(args) == 1 {
			id = args[0]
		}
		legacy, _ := cmd.Flags().GetString("id")
		if id != "" && legacy != "" && id != legacy {
			return failure("argument_conflict", "Choose one resource id")
		}
		if id == "" {
			id = legacy
		}
		if verb != "list" && id == "" {
			return failure("missing_argument", "Supply one "+kind+" id")
		}
		if len(args) > 1 || verb == "list" && len(args) > 0 {
			return failure("unexpected_argument", "Unexpected resource argument")
		}
		rows, err := c.resourceDirectory(cmd.Context(), kind)
		if err != nil {
			return failure("catalog_unavailable", "Local resource directory could not be read")
		}
		if verb == "list" {
			next = resourceScope(cmd, "drivebridge "+kind+" add --help")
			if len(rows) > 0 {
				next = resourceScope(cmd, "drivebridge "+kind+" describe '"+strings.ReplaceAll(rows[0]["id"].(string), "'", "'\"'\"'")+"'")
			}
			data := map[string]any{"count": len(rows), "result_count": len(rows), kind + "s": rows, "next_command": next}
			for i, row := range rows {
				data[fmt.Sprintf("%s.%d.id", kind, i)] = row["id"]
			}
			if c.mode.name() == "summary" {
				visible := rows
				all, _ := cmd.Flags().GetBool("all")
				if !all && len(visible) > 20 {
					visible = visible[:20]
				}
				if len(visible) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "ID")
				}
				for _, r := range visible {
					fmt.Fprintln(cmd.OutOrStdout(), r["id"])
				}
				if len(rows) > len(visible) {
					fmt.Fprintf(cmd.OutOrStdout(), "%d more hidden — %s\n", len(rows)-len(visible), resourceScope(cmd, "drivebridge "+kind+" list --all"))
				}
			}
			return c.mode.write(kind+".list", "success", fmt.Sprintf("%d %s(s)", len(rows), kind), data)
		}
		for _, r := range rows {
			if r["id"] == id {
				data := r
				if verb == "get" {
					data = map[string]any{"id": id, kind + "_id": id}
					for _, key := range []string{"label", "state", "status", "instance_id", "credential_configured", "revoked_at"} {
						if v, ok := r[key]; ok {
							data[key] = v
						}
					}
				}
				data["next_command"] = resourceScope(cmd, "drivebridge "+kind+" list")
				if c.mode.name() == "summary" || c.mode.name() == "explain" {
					keys := make([]string, 0, len(data))
					for key := range data {
						if key != "next_command" && key != "id" {
							keys = append(keys, key)
						}
					}
					sort.Strings(keys)
					fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\n", id)
					for _, key := range keys {
						fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", key, data[key])
					}
				}
				return c.mode.write(kind+"."+verb, "success", fmt.Sprintf("%s %s", kind, id), data)
			}
		}
		return failure("unknown_"+kind, fmt.Sprintf("Unknown %s %q", kind, id))
	}
	return cmd
}

func resourceCompletionPreRun(cmd *cobra.Command) bool {
	if strings.HasPrefix(cmd.Name(), "__complete") {
		cmd.Root().SetErr(io.Discard)
		return true
	}
	return false
}
