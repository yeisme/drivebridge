package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func newBindCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "bind", Short: "Bind a project to a DriveBridge space prefix (caller labels are not authentication)."}
	root.AddCommand(bindDiscoverCommand(c), bindAddCommand(c), bindListCommand(c), bindShowCommand(c), bindRevokeCommand(c))
	return root
}

func bindDiscoverCommand(c *cli) *cobra.Command {
	var allowGeneric bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "List spaces eligible for bind; default is bind_default kinds and adopted spaces.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			rows := s.DiscoverBindSpaces(allowGeneric)
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, row := range rows {
					fmt.Printf("%-16s %-10s adopted=%t bind_default=%t\n", row.Space, row.Kind, row.Adopted, row.BindDefault)
				}
			}
			return c.mode.write("bind.discover", "success", fmt.Sprintf("%d bind candidate space(s).", len(rows)), map[string]any{"count": len(rows), "spaces": rows})
		},
	}
	cmd.Flags().BoolVar(&allowGeneric, "allow-generic", false, "Include escape-hatch spaces (openlist/webdav/smb/sftp/alias)")
	return cmd
}

func bindAddCommand(c *cli) *cobra.Command {
	var consumer, project, space, prefix, purpose, connection, key string
	var allowGeneric bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Bind a consumer project to a space prefix and purpose (labels are stored facts, not authentication).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			missing := []string{}
			if consumer == "" {
				missing = append(missing, "--consumer")
			}
			if project == "" {
				missing = append(missing, "--project")
			}
			if space == "" {
				missing = append(missing, "--space")
			}
			if !cmd.Flags().Changed("prefix") {
				missing = append(missing, "--prefix")
			}
			if purpose == "" {
				missing = append(missing, "--purpose")
			}
			if len(missing) > 0 {
				return fail("bind_missing_flag", "bind add is missing: "+joinComma(missing)+". Caller labels are not authentication.")
			}
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			rec, reused, err := s.AddBind(bridge.BindRequest{
				Consumer: consumer, ProjectRef: project, Space: space, Prefix: prefix,
				Purpose: purpose, ConnectionID: connection, IdempotencyKey: key, AllowGeneric: allowGeneric,
			})
			if err != nil {
				if err.Error() == "bind_escape_denied" {
					return fail("bind_escape_denied", "That space is an escape hatch; bind an adopted local/S3 space or OneDrive/Google Drive, or pass --allow-generic.")
				}
				return err
			}
			summary := fmt.Sprintf("Bound %s/%s to space %s prefix %q purpose %s.", rec.Consumer, rec.ProjectRef, rec.SpaceID, rec.Prefix, rec.Purpose)
			if reused {
				summary = fmt.Sprintf("Reused bind of %s/%s to space %s prefix %q.", rec.Consumer, rec.ProjectRef, rec.SpaceID, rec.Prefix)
			}
			return c.mode.write("bind-add", "success", summary, bindEnvelope(*rec, reused))
		},
	}
	f := cmd.Flags()
	f.StringVar(&consumer, "consumer", "", "Consumer identifier (stored fact, not authentication)")
	f.StringVar(&project, "project", "", "Project reference (stored fact, not authentication)")
	f.StringVar(&space, "space", "", "Existing space identifier")
	f.StringVar(&prefix, "prefix", "", "Allowed workspace-relative prefix (empty = whole space)")
	f.StringVar(&purpose, "purpose", "", "Bind purpose: vault, notes-attachments, materials, saveback, owner-store")
	f.StringVar(&connection, "connection", "", "Optional connection_id to record (never prints credentials)")
	f.StringVar(&key, "idempotency-key", "", "Stable idempotency key (default: bind:<consumer>:<project>:<space>:<prefix>:<purpose>)")
	f.BoolVar(&allowGeneric, "allow-generic", false, "Allow binding an escape-hatch space")
	return cmd
}

func bindListCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List local consumer bind records (no credentials).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			rows := s.Binds()
			data := make([]map[string]any, 0, len(rows))
			for _, rec := range rows {
				data = append(data, bindEnvelope(rec, false))
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, rec := range rows {
					revoked := ""
					if rec.RevokedAt != nil {
						revoked = " revoked"
					}
					fmt.Printf("%s %-12s %-16s %-16s %s %q%s\n", rec.ID, rec.Consumer, rec.ProjectRef, rec.SpaceID, rec.Purpose, rec.Prefix, revoked)
				}
			}
			return c.mode.write("bind.list", "success", fmt.Sprintf("%d bind record(s).", len(rows)), map[string]any{"count": len(rows), "binds": data})
		},
	}
}

func bindShowCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "show <bind-id>",
		Short: "Show one bind record (no credentials).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			rec, err := s.Bind(args[0])
			if err != nil {
				return err
			}
			return c.mode.write("bind.show", "success", fmt.Sprintf("Bind %s %s/%s -> %s.", rec.ID, rec.Consumer, rec.ProjectRef, rec.SpaceID), bindEnvelope(rec, false))
		},
	}
}

func bindRevokeCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <bind-id>",
		Short: "Revoke a bind so new scoped reads fail; does not delete the space, adopt record, or owner files.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			rec, err := s.RevokeBind(args[0])
			if err != nil {
				return err
			}
			data := bindEnvelope(rec, false)
			data["revoked"] = true
			data["owner_files_deleted"] = false
			data["space_deleted"] = false
			data["adopt_deleted"] = false
			return c.mode.write("bind.revoke", "success", fmt.Sprintf("Bind %s revoked; new list/stat/import for it are refused. Owner files, the space and adopt records were not deleted.", rec.ID), data)
		},
	}
}

func bindEnvelope(rec bridge.BindRecord, reused bool) map[string]any {
	out := map[string]any{
		"bind_id": rec.ID, "consumer": rec.Consumer, "project_ref": rec.ProjectRef,
		"space": rec.SpaceID, "prefix": rec.Prefix, "purpose": rec.Purpose,
		"revoked": rec.RevokedAt != nil,
	}
	if rec.IdempotencyKey != "" {
		out["idempotency_key"] = rec.IdempotencyKey
	}
	if rec.ConnectionID != "" {
		out["connection_id"] = rec.ConnectionID
	}
	if reused {
		out["reused"] = true
	}
	return out
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func bindContext(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if cmd.Flags().Lookup("bind") == nil {
		return ctx
	}
	bind, _ := cmd.Flags().GetString("bind")
	consumer, _ := cmd.Flags().GetString("consumer")
	project, _ := cmd.Flags().GetString("project")
	return bridge.WithBindScope(ctx, bridge.BindScope{ID: bind, Consumer: consumer, ProjectRef: project})
}

func addBindScopeFlags(cmd *cobra.Command) {
	cmd.Flags().String("consumer", "", "Caller-declared consumer label (stored fact, not authentication)")
	cmd.Flags().String("project", "", "Caller-declared project reference (stored fact, not authentication)")
	cmd.Flags().String("bind", "", "Bind identifier that scopes this operation to its prefix")
}
