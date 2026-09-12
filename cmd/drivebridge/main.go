package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		// All surfaced errors here are our typed codes or wrapped config
		// errors; third-party raw errors are discarded further down.
		if !errors.Is(err, errDoctorNotReady) {
			fmt.Fprintf(os.Stderr, "DriveBridge failed: %v. Check local configuration and the documented recovery steps.\n", err)
		}
		os.Exit(1)
	}
}

// newRootCommand assembles the cobra tree. Commands are registered through
// this name map so the executable surface stays readable in one place.
func newRootCommand() *cobra.Command {
	c := &cli{}
	root := &cobra.Command{
		Use:           "drivebridge",
		Short:         "Local-first personal file MCP over OneDrive and any rclone backend.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			c.command = cmd.Name()
			file, effective, err := c.resolve(cmd)
			if err != nil {
				return err
			}
			c.effective = effective
			c.file = file
			return nil
		},
	}
	c.root = root
	pf := root.PersistentFlags()
	pf.StringVar(&c.configPath, "config", "", "Explicit configuration file (discovery: this flag, DRIVEBRIDGE_CONFIG, ./drivebridge.*, user config dir)")
	pf.StringVar(&c.flagState, "state", "", "Private local state directory")
	pf.StringVar(&c.flagRclone, "rclone-config", "", "Existing rclone configuration; credentials are never printed")
	pf.BoolVar(&c.mode.JSON, "json", false, "Print the machine envelope")
	pf.BoolVar(&c.mode.Agent, "agent", false, "Print stable key=value status")
	pf.BoolVar(&c.mode.Events, "events", false, "Print start/end NDJSON events")
	pf.BoolVar(&c.mode.Explain, "explain", false, "Explain the local result")

	for name, command := range map[string]*cobra.Command{
		"auth":         newAuthCommand(c),
		"doctor":       newDoctorCommand(c),
		"capabilities": newCapabilitiesCommand(c),
		"mcp":          newMCPCommand(c),
		"remote":       newRemoteCommand(c),
		"space":        newSpaceCommand(c),
		"space-add":    newSpaceAddCommand(c),
		"config":       newConfigCommand(c),
		"install":      newInstallCommand(c),
	} {
		root.AddCommand(command)
		_ = name
	}
	for _, command := range newTransferCommands(c) {
		root.AddCommand(command)
	}
	root.InitDefaultCompletionCmd()
	return root
}
