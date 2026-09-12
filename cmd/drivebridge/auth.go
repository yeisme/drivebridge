package main

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/fs/config"
	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// newAuthCommand is the one-command first-use path: a device-code sign-in
// against the rclone public OneDrive client, with optional drive/root
// discovery so activation needs no hand-copied identifiers. The legacy rclone
// interactive configurator stays behind --rclone-interactive. Tokens are never
// printed; the sign-in prompt carries only the verification page and code.
func newAuthCommand(c *cli) *cobra.Command {
	var interactive, auto bool
	var remote, rootName string
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Sign in to OneDrive with a one-time device code; discovers drive and root on request.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.installRclone(); err != nil {
				return err
			}
			if interactive {
				// The configurator is interactive by design; it runs in the
				// user terminal only and never prints through the output
				// contract.
				return rcloneconfig.EditConfig(cmd.Context())
			}
			if remote == "" {
				remote = "onedrive"
			}
			if config.GetValue(remote, "token") != "" {
				return fmt.Errorf("remote_exists: %s already holds a credential; use a new --remote name or remove the old section first", remote)
			}
			device := bridge.NewDeviceAuth()
			code, err := device.Start(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Sign in at %s using the code: %s\n", code.VerificationURI, code.UserCode)
			if err = device.Await(cmd.Context(), remote, code); err != nil {
				return err
			}
			data := map[string]any{"remote": remote, "authenticated": true}
			if auto || rootName != "" {
				driveID, rootID, e := device.DiscoverDefaultDrive(cmd.Context())
				if e != nil {
					return e
				}
				if rootName != "" {
					selected, e := device.SelectRootFolder(cmd.Context(), rootName)
					if e != nil {
						return e
					}
					rootID = selected
				}
				if e = device.SetDrive(remote, driveID, rootID); e != nil {
					return e
				}
				data["drive_id"], data["root_id"] = driveID, rootID
				if rootName != "" {
					data["root_name"] = rootName
				}
			}
			return c.mode.write("auth", "success", fmt.Sprintf("Signed in; credential stored in the rclone config as %s.", remote), data)
		},
	}
	cmd.Flags().BoolVar(&interactive, "rclone-interactive", false, "Open the legacy rclone interactive configurator instead")
	cmd.Flags().StringVar(&remote, "remote", "onedrive", "rclone remote name to create or replace")
	cmd.Flags().BoolVar(&auto, "auto", false, "Discover the default drive and its root after sign-in")
	cmd.Flags().StringVar(&rootName, "root-name", "", "Select the allowed root folder by exact name under the drive root")
	return cmd
}
