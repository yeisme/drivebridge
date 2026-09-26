package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c := &cli{}
	if err := newRootCommand(c).ExecuteContext(ctx); err != nil {
		if errors.Is(err, errQuiet) || errors.Is(err, errDoctorNotReady) {
			// The command already produced its full projection.
			os.Exit(1)
		}
		command := c.command
		if command == "" {
			command = "cli"
		}
		code, message := codify(err)
		_ = c.mode.fail(command, code, message, nil)
		if code == "command_failed" {
			fmt.Fprintln(os.Stderr, "DriveBridge failed. Check local configuration and the documented recovery steps.")
		}
		os.Exit(1)
	}
}

// newRootCommand assembles the cobra tree. Commands are registered through
// this name map so the executable surface stays readable in one place.
func newRootCommand(c *cli) *cobra.Command {
	root := &cobra.Command{
		Use:           "drivebridge",
		Short:         "Local-first personal file MCP over OneDrive and any rclone backend.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			c.command = commandAction(cmd)
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
	pf.String("aigora-url", "", "Aigora origin for unified remote authentication")
	pf.StringVar(&c.configPath, "config", "", "Explicit configuration file (discovery: this flag, DRIVEBRIDGE_CONFIG, ./drivebridge.*, user config dir)")
	pf.StringVar(&c.flagState, "state", "", "Private local state directory")
	pf.StringVar(&c.flagRclone, "rclone-config", "", "Existing rclone configuration; credentials are never printed")
	pf.BoolVar(&c.mode.JSON, "json", false, "Print the machine envelope")
	pf.BoolVar(&c.mode.Agent, "agent", false, "Print stable key=value status")
	pf.BoolVar(&c.mode.Events, "events", false, "Print start/end NDJSON events")
	pf.BoolVar(&c.mode.Explain, "explain", false, "Explain the local result")

	for name, command := range map[string]*cobra.Command{
		"relay":        newRelayCommand(c),
		"view":         newViewCommand(c),
		"usage":        newUsageCommand(c),
		"audit":        newAuditCommand(c),
		"auth":         newAuthCommand(c),
		"resume":       newRecoveryCommand(c, "resume"),
		"cancel":       newRecoveryCommand(c, "cancel"),
		"recent":       newRecentCommand(c),
		"setup":        newSetupCommand(c),
		"doctor":       newDoctorCommand(c),
		"pin":          newPinCommand(c),
		"gc":           newGCCommand(c),
		"retention":    newRetentionCommand(c),
		"link":         newLinkCommand(c),
		"sync":         newSyncCommand(c),
		"capabilities": newCapabilitiesCommand(c),
		"mcp":          newMCPCommand(c),
		"mount":        newMountCommand(c),
		"preset":       newPresetCommand(c),
		"storage":      newStorageCommand(c),
		"remote":       newRemoteCommand(c),
		"space":        newSpaceCommand(c),
		"connection":   newConnectionCommand(c),
		"bind":         newBindCommand(c),
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

// commandAction derives the envelope command identity from the full command
// path so failures carry the same identity as successes (space.add is
// normalized to the legacy hyphenated action space-add, likewise storage).
func commandAction(cmd *cobra.Command) string {
	action := strings.TrimPrefix(cmd.CommandPath(), "drivebridge ")
	action = strings.ReplaceAll(action, " ", ".")
	switch action {
	case "space.add":
		return "space-add"
	case "storage.add":
		return "storage-add"
	case "bind.add":
		return "bind-add"
	}
	return action
}
