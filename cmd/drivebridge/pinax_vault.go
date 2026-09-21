package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
	"github.com/yeisme/drivebridge/internal/recovery"
)

func applyPinaxVaultOverlay(c *cli, spaceID, vaultRoot string) (bridge.PinaxVaultReceipt, error) {
	var zero bridge.PinaxVaultReceipt
	if spaceID == "" || vaultRoot == "" {
		return zero, fail("storage_missing_flag", "pinax-vault requires --space and --vault-root.")
	}
	s, release, err := c.openService(false)
	if err != nil {
		return zero, err
	}
	defer release()
	space, err := s.Space(spaceID)
	if err != nil {
		return zero, fail(err.Error(), recovery.Hint(err.Error()))
	}
	content, err := bridge.PrepareSpaceContentRoot(space, c.effective.Values["state"], c.effective.Values["rclone_config"])
	if err != nil {
		return zero, fail(err.Error(), recovery.Hint(err.Error()))
	}
	failed := true
	defer func() {
		if failed {
			content.Close()
		}
	}()
	receipt, err := bridge.ApplyPinaxOverlay(vaultRoot, content.Path, space.ID)
	if err != nil {
		return zero, fail(err.Error(), recovery.Hint(err.Error()))
	}
	receipt, err = receipt.WithAdapter(content.Adapter)
	if err != nil {
		bridge.RemovePinaxOverlay(receipt)
		return zero, fail(err.Error(), recovery.Hint(err.Error()))
	}
	st := bridge.MountState{
		Adapter:     receipt.Adapter,
		Mode:        "writable",
		SpaceIDs:    []string{space.ID},
		StartedAt:   time.Now().UTC(),
		Preset:      receipt.Preset,
		VaultRoot:   receipt.VaultRoot,
		Overlay:     receipt.Overlay,
		ContentRoot: receipt.ContentRoot,
		Links:       receipt.Links,
	}
	if err := s.RegisterMount(st); err != nil {
		bridge.RemovePinaxOverlay(receipt)
		return zero, fail(err.Error(), recovery.Hint(err.Error()))
	}
	failed = false
	return receipt, nil
}

func newPresetCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "preset", Short: "Apply a product overlay recipe (Pinax vault path mount)"}
	var space, vaultRoot string
	pinax := &cobra.Command{
		Use:   "pinax-vault",
		Short: "Overlay an existing space onto a local Pinax vault-root; does not run pinax",
		RunE: func(cmd *cobra.Command, _ []string) error {
			receipt, err := applyPinaxVaultOverlay(c, space, vaultRoot)
			if err != nil {
				return err
			}
			next := []string{
				fmt.Sprintf("pinax init %s --title \"Cloud notes\" --json", receipt.VaultRoot),
				fmt.Sprintf("pinax vault register %s --name cloud --default", receipt.VaultRoot),
				fmt.Sprintf("pinax storage set local --root %s --vault %s --json", receipt.VaultRoot, receipt.VaultRoot),
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				fmt.Fprintf(cmd.OutOrStdout(), "Pinax vault overlay ready at %s.\nNext (Pinax, local only):\n  %s\n  %s\n  %s\n", receipt.VaultRoot, next[0], next[1], next[2])
			}
			return c.mode.write("preset.pinax-vault", "success", "Pinax vault overlay is ready at "+receipt.VaultRoot+".", map[string]any{
				"preset": receipt.Preset, "vault_root": receipt.VaultRoot, "writable": true,
				"overlay": receipt.Overlay, "adapter": receipt.Adapter, "space": receipt.Space,
				"next": next,
			})
		},
	}
	pinax.Flags().StringVar(&space, "space", "", "Existing DriveBridge space id")
	pinax.Flags().StringVar(&vaultRoot, "vault-root", "", "Absolute local vault directory")
	_ = pinax.MarkFlagRequired("space")
	_ = pinax.MarkFlagRequired("vault-root")
	root.AddCommand(pinax)
	return root
}
