package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// connectionProjection is the consumer-facing read-only view. Credentials,
// token file paths and raw transport details never appear here.
func connectionProjection(cn bridge.Connection, configured bool, synthetic bool) map[string]any {
	return map[string]any{
		"connection_id":         cn.ID,
		"label":                 cn.Label,
		"base_url":              cn.BaseURL,
		"instance_id":           cn.InstanceID,
		"space_id":              cn.SpaceID,
		"project_ref":           cn.ProjectRef,
		"credential_source":     cn.CredentialSource,
		"credential_configured": configured,
		"synthetic":             synthetic,
	}
}

// credentialConfigured reports whether the declared credential reference
// resolves, without reading or printing the credential itself.
func credentialConfigured(cn bridge.Connection) bool {
	switch cn.CredentialSource {
	case "env":
		return os.Getenv("DRIVEBRIDGE_TOKEN") != ""
	case "token-file":
		info, err := os.Lstat(cn.TokenFile)
		return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0
	default:
		// local-config credentials live in the rclone configuration owned by
		// 'drivebridge auth'; presence of the record itself is the reference.
		return true
	}
}

func newConnectionCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "connection", Short: "Manage named DriveBridge connections."}

	var label, baseURL, instanceID, spaceID, projectRef, credSource, tokenFile string
	add := &cobra.Command{
		Use:   "add",
		Short: "Register a named connection (writes only through the CLI).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			cn := bridge.Connection{ID: id, Label: label, BaseURL: baseURL, InstanceID: instanceID, SpaceID: spaceID, ProjectRef: projectRef, CredentialSource: credSource, TokenFile: tokenFile}
			if err = s.AddConnection(cn); err != nil {
				return err
			}
			return c.mode.write("connection.add", "success", fmt.Sprintf("Connection %s registered; connectivity has not been verified.", id), connectionProjection(cn, credentialConfigured(cn), false))
		},
	}
	add.Flags().String("id", "", "Connection identifier (slug)")
	add.Flags().StringVar(&label, "label", "", "Human-readable label")
	add.Flags().StringVar(&baseURL, "base-url", "", "DriveBridge service origin (http/https)")
	add.Flags().StringVar(&instanceID, "instance-id", "", "Service instance identifier")
	add.Flags().StringVar(&spaceID, "space-id", "", "Existing space identifier to bind")
	add.Flags().StringVar(&projectRef, "project-ref", "", "External project reference")
	add.Flags().StringVar(&credSource, "credential-source", "", "Credential source: local-config, env or token-file")
	add.Flags().StringVar(&tokenFile, "token-file", "", "Private token file reference (never printed)")

	set := &cobra.Command{
		Use:   "set",
		Short: "Update fields of an existing connection.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			patch := bridge.Connection{Label: label, BaseURL: baseURL, InstanceID: instanceID, SpaceID: spaceID, ProjectRef: projectRef, CredentialSource: credSource, TokenFile: tokenFile}
			if err = s.SetConnection(id, patch); err != nil {
				return err
			}
			cn, _ := s.Connection(id)
			return c.mode.write("connection.set", "success", fmt.Sprintf("Connection %s updated.", id), connectionProjection(cn, credentialConfigured(cn), false))
		},
	}
	set.Flags().String("id", "", "Connection identifier")
	set.Flags().StringVar(&label, "label", "", "Human-readable label")
	set.Flags().StringVar(&baseURL, "base-url", "", "DriveBridge service origin (http/https)")
	set.Flags().StringVar(&instanceID, "instance-id", "", "Service instance identifier")
	set.Flags().StringVar(&spaceID, "space-id", "", "Existing space identifier to bind")
	set.Flags().StringVar(&projectRef, "project-ref", "", "External project reference")
	set.Flags().StringVar(&credSource, "credential-source", "", "Credential source: local-config, env or token-file")
	set.Flags().StringVar(&tokenFile, "token-file", "", "Private token file reference (never printed)")

	remove := &cobra.Command{
		Use:   "remove",
		Short: "Soft-delete a connection; referenced spaces and files are untouched.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			if err = s.RemoveConnection(id); err != nil {
				return err
			}
			return c.mode.write("connection.remove", "success", fmt.Sprintf("Connection %s removed.", id), map[string]any{"connection_id": id, "removed": true})
		},
	}
	remove.Flags().String("id", "", "Connection identifier")

	list := &cobra.Command{
		Use:   "list",
		Short: "List connections as a read-only projection (no credentials).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			connections := s.Connections()
			data := make([]map[string]any, 0, len(connections)+1)
			for _, cn := range connections {
				data = append(data, connectionProjection(cn, credentialConfigured(cn), false))
			}
			// Empty registry: project the legacy global remote configuration as
			// a synthetic default connection without persisting anything.
			if len(connections) == 0 {
				if url := c.effective.Values["remote_url"]; url != "" {
					source := "env"
					configured := os.Getenv("DRIVEBRIDGE_TOKEN") != ""
					if tokenPath := c.effective.Values["remote_token_file"]; tokenPath != "" {
						source = "token-file"
						configured = credentialConfigured(bridge.Connection{CredentialSource: "token-file", TokenFile: tokenPath})
					}
					data = append(data, connectionProjection(bridge.Connection{ID: "default", Label: "Default (global remote_url)", BaseURL: url, InstanceID: "local", CredentialSource: source}, configured, true))
				}
			}
			summary := fmt.Sprintf("%d connection(s).", len(data))
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, row := range data {
					fmt.Printf("%-16s %-24s %s\n", row["connection_id"], row["label"], row["base_url"])
				}
			}
			return c.mode.write("connection.list", "success", summary, map[string]any{"count": len(data), "connections": data})
		},
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show one connection as a read-only projection (no credentials).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			cn, err := s.Connection(id)
			if err != nil {
				return err
			}
			return c.mode.write("connection.show", "success", fmt.Sprintf("Connection %s (%s).", cn.ID, cn.Label), connectionProjection(cn, credentialConfigured(cn), false))
		},
	}
	show.Flags().String("id", "", "Connection identifier")

	root.AddCommand(add, set, remove, list, show)
	return root
}
