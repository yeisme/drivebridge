package main

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yeisme/drivebridge/internal/desktop"
)

func newDesktopCommand() *cobra.Command {
	root := &cobra.Command{Use: "desktop", Short: "Private foreground desktop host; metadata preflight only."}
	// This opt-in surface must not discover cloud/config state before receiving
	// its private native-host bootstrap. It owns a separate stdout protocol.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		invalid := false
		cmd.Flags().Visit(func(flag *pflag.Flag) {
			if flag.Name != "help" {
				invalid = true
			}
		})
		if invalid {
			return fail("desktop_flags_invalid", "Desktop host commands do not accept inherited configuration or output flags.")
		}
		return nil
	}
	root.AddCommand(&cobra.Command{Use: "serve", Short: "Read private native-host frames from stdin; stop on EOF.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		executable, err := os.Executable()
		if err != nil {
			return fail("desktop_start_failed", "The packaged executable is unavailable.")
		}
		// CLI owns actual OS pipes; no shell, PATH lookup or secret-bearing flags.
		if err := desktop.ServeHost(cmd.Context(), os.Stdin, os.Stdout, executable); err != nil {
			return errQuiet
		}
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "schema", Short: "Export private host schemas; never use these in a renderer.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(desktop.HostSchemas())
	}})
	return root
}
