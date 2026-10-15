package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	c := &cli{}
	if err := newRootCommand(c).Execute(); err != nil {
		// Doctor's not-ready report is a complete projection with its own
		// exit contract (2); quiet sentinels already emitted everything.
		if errors.Is(err, errDoctorNotReady) {
			os.Exit(2)
		}
		if errors.Is(err, errQuiet) {
			os.Exit(1)
		}
		command := c.command
		if command == "" {
			command = "cli"
		}
		code, message := codify(err)
		_ = c.mode.emitFailure(command, code, message, nil)
		if code == "command_failed" {
			fmt.Fprintln(os.Stderr, "DriveBridge server failed. Check local configuration and redacted evidence.")
		}
		os.Exit(1)
	}
}

// newRootCommand assembles the cobra tree; each command resolves its keys
// through the shared configuration surface (flag > env > file > default).
func newRootCommand(c *cli) *cobra.Command {
	root := &cobra.Command{
		Use:           "drivebridge-server",
		Short:         "Single-tenant personal file infrastructure server.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			c.command = cmd.Name()
			return c.resolve(cmd)
		},
	}
	c.root = root
	pf := root.PersistentFlags()
	pf.StringVar(&c.configPath, "config", "", "Explicit configuration file (discovery: this flag, DRIVEBRIDGE_SERVER_CONFIG, ./drivebridge-server.*, user config dir, /etc/drivebridge-server)")
	pf.StringVar(&c.flagState, "state", "", "Private state directory")
	pf.BoolVar(&c.mode.json, "json", false, "Print a JSON result for configuration or backup")
	pf.BoolVar(&c.mode.agent, "agent", false, "Print key=value status")
	pf.BoolVar(&c.mode.events, "events", false, "Print NDJSON status events")
	pf.BoolVar(&c.mode.explain, "explain", false, "Explain the local result")

	// Pointers only: copying cobra.Command values would detach subcommand
	// parent links and break inherited persistent flags.
	for _, command := range []*cobra.Command{
		newServeCommand(c),
		newInstanceAuditCommand(c),
		newPlatformConnectCommand(c),
		newUsersCommand(c),
		newGovernanceCommand(c),
		newSpacesCommand(c),
		newBlobCommand(c),
		newHandoffBindingCommand(c),
		newHandoffCommand(c),
		newAgentsCommand(c),
		newStatusCommand(c),
		newBackupCommand(c),
		newKeyRotateCommand(c),
		newSpaceExportCommand(c),
		newMetadataCommand(c),
		newConfigCommand(c),
	} {
		root.AddCommand(command)
	}
	root.AddCommand(newDoctorCommand(c))
	root.InitDefaultCompletionCmd()
	return root
}
