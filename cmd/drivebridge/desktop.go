package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"github.com/yeisme/drivebridge/internal/bridge"
	"github.com/yeisme/drivebridge/internal/desktop"
)

// desktopOwnerAdmission attaches the local owner guard to the desktop host. It
// runs only after the private bootstrap frame has been validated, and it never
// creates or migrates a state: without an existing owner database the host stays
// deny-only. The state path comes from configuration that predates the process,
// never from a renderer request or an inherited flag.
func desktopOwnerAdmission() (desktop.Admission, func(), error) {
	state, ok := ownerStatePath()
	if !ok {
		return nil, nil, nil
	}
	return desktopOwnerAdmissionAt(state)
}

func desktopOwnerAdmissionAt(state string) (desktop.Admission, func(), error) {
	if info, err := os.Lstat(state); err != nil || !info.IsDir() {
		return nil, nil, nil
	}
	if _, err := os.Lstat(filepath.Join(state, "state.sqlite")); err != nil {
		return nil, nil, nil
	}
	lock, err := os.OpenFile(filepath.Join(state, "process.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, nil
	}
	release := func() {
		_ = lockFile(lock, lockRelease)
		lock.Close()
	}
	if err = lockFile(lock, lockExclusive|lockNonBlocking); err != nil {
		lock.Close()
		return nil, nil, nil
	}
	service, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		release()
		return nil, nil, nil
	}
	admission, err := desktop.NewBridgeAdmission(service)
	if err != nil {
		service.Close()
		release()
		return nil, nil, nil
	}
	return admission, func() { service.Close(); release() }, nil
}

func newDesktopCommand() *cobra.Command {
	root := &cobra.Command{Use: "desktop", Short: "Private foreground desktop host for guarded local reading."}
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
		if err := desktop.ServeHostWithState(cmd.Context(), os.Stdin, os.Stdout, executable, desktopSelectedAdmission); err != nil {
			return errQuiet
		}
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "schema", Short: "Export private host schemas; never use these in a renderer.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(desktop.HostSchemas())
	}})
	return root
}

func desktopSelectedAdmission(input desktop.HostBootstrap) (desktop.Admission, func(), string, error) {
	state := input.StatePath
	if state == "" {
		var ok bool
		state, ok = ownerStatePath()
		if !ok {
			return nil, nil, "", nil
		}
	}
	if input.StateAction != "" {
		if err := desktopPrepareState(state, input.StateAction); err != nil {
			return nil, nil, "", err
		}
	}
	layout, err := managedstate.Inspect(state, "cli")
	if err != nil {
		return nil, nil, "", err
	}
	// Explicit state selection requires a registered layout; legacy migration is
	// a distinct native action, never a fallback during opening.
	if input.StatePath != "" && layout.Layout != "fenced" {
		return nil, nil, "", nil
	}
	admission, release, err := desktopOwnerAdmissionAt(state)
	return admission, release, layout.Plan.StateIdentity, err
}

// Only the private native bootstrap can request these explicit state actions.
// Failed preparation leaves its state available for diagnosis, never replaces
// an existing directory or silently opens another database.
func desktopPrepareState(state, action string) error {
	if !filepath.IsAbs(state) || filepath.Clean(state) != state {
		return errors.New("state_path_invalid")
	}
	if action == "create" {
		if err := os.Mkdir(state, 0700); err != nil {
			return errors.New("state_destination_exists_or_unavailable")
		}
		lock, err := os.OpenFile(filepath.Join(state, "process.lock"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return errors.New("state_unavailable")
		}
		if err = lockFile(lock, lockExclusive|lockNonBlocking); err != nil {
			lock.Close()
			return errors.New("state_busy")
		}
		service, openErr := bridge.New(state, bridge.NewOneDrive(), false)
		if service != nil {
			service.Close()
		}
		_ = lockFile(lock, lockRelease)
		lock.Close()
		if openErr != nil {
			return openErr
		}
	} else if action != "migrate" {
		return errors.New("state_action_invalid")
	}
	layout, err := managedstate.Inspect(state, "cli")
	if err != nil {
		return err
	}
	if layout.Layout == "fenced" {
		return nil
	}
	if layout.Layout != "legacy" {
		return errors.New("state_layout_invalid")
	}
	plan, err := managedstate.Prepare(state, "cli", "desktop-native-migration")
	if err != nil {
		return err
	}
	_, err = managedstate.Install(state, plan)
	return err
}
