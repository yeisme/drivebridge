package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func loopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newMountCommand(c *cli) *cobra.Command {
	mount := &cobra.Command{Use: "mount", Short: "Mount authorized spaces over WebDAV (loopback, read-only by default)"}

	webdav := &cobra.Command{
		Use:   "webdav",
		Short: "Serve the VFS over WebDAV; read-only unless --writable, loopback unless --lan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			spaces, _ := cmd.Flags().GetStringArray("space")
			listen, _ := cmd.Flags().GetString("listen")
			lan, _ := cmd.Flags().GetBool("lan")
			writable, _ := cmd.Flags().GetBool("writable")
			if !loopbackListen(listen) && !lan {
				return fail("listen_not_allowed", "Non-loopback listen requires the explicit --lan flag.")
			}
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			if len(spaces) == 0 {
				for _, p := range s.Spaces() {
					spaces = append(spaces, p.ID)
				}
				if len(spaces) == 0 {
					return fail("invalid_space", "No spaces configured; add one with space-add first.")
				}
			}
			for _, id := range spaces {
				if _, err := s.Space(id); err != nil {
					return err
				}
			}
			tokenBytes := make([]byte, 16)
			if _, err := rand.Read(tokenBytes); err != nil {
				return fail("state_unavailable", "Could not mint the mount session token.")
			}
			token := hex.EncodeToString(tokenBytes)
			mode := "readonly"
			if writable {
				mode = "writable"
			}
			listener, err := net.Listen("tcp", listen)
			if err != nil {
				return fail("mount_listener_unavailable", "The requested listen address is unavailable.")
			}
			actual := listener.Addr().String()
			state := bridge.MountState{Adapter: "webdav", Listen: actual, Mode: mode, SpaceIDs: spaces, Token: token, PID: os.Getpid(), StartedAt: time.Now().UTC()}
			if err := s.RegisterMount(state); err != nil {
				listener.Close()
				return err
			}
			defer s.ClearMount(os.Getpid())
			var root *os.Root
			if writable {
				p, _ := s.Space(spaces[0])
				if root, err = os.OpenRoot(p.LocalRoot); err != nil {
					return fail("local_root_unavailable", "The first space's local root is required for writable staging.")
				}
				defer root.Close()
			}
			handler := &bridge.WebDAVHandler{VFS: bridge.NewVFS(s, spaces, writable), Token: token, Root: root}
			server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
			go func() {
				<-cmd.Context().Done()
				server.Close()
			}()
			launchFile := filepath.Join(c.effective.Values["state"], "mount-webdav.url")
			if err := os.WriteFile(launchFile, []byte(fmt.Sprintf("http://drivebridge:%s@%s/\n", token, actual)), 0600); err != nil {
				return fail("state_unavailable", "Could not write the private mount launch file.")
			}
			defer os.Remove(launchFile)
			fmt.Fprintf(cmd.OutOrStdout(), "WebDAV mount serving %d space(s) at %s (%s).\nPrivate launch file: %s\n", len(spaces), actual, mode, launchFile)
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				return fail("mount_listener_unavailable", "The WebDAV listener stopped unexpectedly.")
			}
			return nil
		},
	}
	webdav.Flags().StringArray("space", nil, "Space ID to export (repeatable; default: all configured spaces)")
	webdav.Flags().String("listen", "127.0.0.1:19080", "Listen address; non-loopback requires --lan")
	webdav.Flags().Bool("lan", false, "Allow a non-loopback listen address")
	webdav.Flags().Bool("writable", false, "Enable MKCOL/PUT/MOVE/DELETE mapped to the existing upload/move/trash contracts")

	status := &cobra.Command{
		Use:   "status",
		Short: "Print the read-only mount projection (no token)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			proj := bridge.ReadMountStatus(c.effective.Values["state"])
			summary := "No mount is running."
			if alive, _ := proj["alive"].(bool); alive {
				adapter, _ := proj["adapter"].(string)
				summary = fmt.Sprintf("Mount %s is alive.", adapter)
			} else if adapter, _ := proj["adapter"].(string); adapter != "" {
				summary = fmt.Sprintf("Mount receipt present for %s (alive=false).", adapter)
			}
			return c.mode.write("mount.status", "success", summary, proj)
		},
	}

	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop the CLI-owned mount process for this state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := os.ReadFile(filepath.Join(c.effective.Values["state"], "mount.json"))
			if err != nil {
				return c.mode.write("mount.stop", "success", "No mount is running.", map[string]any{"stopped": false})
			}
			var st bridge.MountState
			if json.Unmarshal(body, &st) != nil {
				return fail("state_unavailable", "The mount receipt is unreadable.")
			}
			if st.Preset == "pinax-vault" || st.VaultRoot != "" {
				bridge.RemovePinaxOverlay(bridge.PinaxVaultReceipt{
					VaultRoot: st.VaultRoot,
					Links:     st.Links,
				})
				if st.Adapter == "pinax-vault-rclone" {
					bridge.UnmountRclonePath(st.ContentRoot, c.effective.Values["rclone_config"])
				}
				_ = os.Remove(filepath.Join(c.effective.Values["state"], "mount.json"))
				return c.mode.write("mount.stop", "success", "Pinax vault overlay stopped; local .pinax and remote content were kept.", map[string]any{"stopped": true, "preset": st.Preset, "vault_root": st.VaultRoot, "adapter": st.Adapter})
			}
			process, err := os.FindProcess(st.PID)
			if err != nil {
				return fail("mount_stop_unavailable", "The mount process is not addressable from this platform.")
			}
			if err := process.Signal(os.Interrupt); err != nil {
				return fail("mount_stop_unavailable", "Stopping the mount process is not supported here; close its terminal instead.")
			}
			return c.mode.write("mount.stop", "success", "Stop signal sent to the mount process.", map[string]any{"stopped": true, "pid": st.PID})
		},
	}

	fuse := &cobra.Command{
		Use:   "fuse",
		Short: "Kernel FUSE mount (not in the default build)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fail("unsupported", "The default CGO_ENABLED=0 build has no FUSE; use mount webdav, or point rclone mount at the WebDAV URL.")
		},
	}

	var vaultPreset, vaultSpace, vaultRoot string
	var vaultWritable bool
	vault := &cobra.Command{
		Use:   "vault",
		Short: "Present a space as a local Pinax vault-root overlay (path, not WebDAV URL)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if vaultPreset != "pinax" && vaultPreset != "pinax-vault" {
				return fail("unsupported", "mount vault currently supports --preset pinax only.")
			}
			if !vaultWritable {
				return fail("content_not_writable", "The pinax vault overlay is a writable notes tree; pass --writable.")
			}
			receipt, err := applyPinaxVaultOverlay(c, vaultSpace, vaultRoot)
			if err != nil {
				return err
			}
			next := fmt.Sprintf("pinax init %s && pinax storage set local --root %s --vault %s", receipt.VaultRoot, receipt.VaultRoot, receipt.VaultRoot)
			return c.mode.write("mount.vault", "success", "Pinax vault overlay is ready at "+receipt.VaultRoot+".", map[string]any{
				"preset": receipt.Preset, "vault_root": receipt.VaultRoot, "writable": true,
				"overlay": receipt.Overlay, "adapter": receipt.Adapter, "space": receipt.Space,
				"next": next,
			})
		},
	}
	vault.Flags().StringVar(&vaultPreset, "preset", "pinax", "Overlay preset (pinax)")
	vault.Flags().StringVar(&vaultSpace, "space", "", "Existing DriveBridge space id")
	vault.Flags().StringVar(&vaultRoot, "vault-root", "", "Absolute local vault directory")
	vault.Flags().BoolVar(&vaultWritable, "writable", true, "Required for pinax overlay (default true)")
	_ = vault.MarkFlagRequired("space")
	_ = vault.MarkFlagRequired("vault-root")

	for _, sub := range []*cobra.Command{webdav, vault, status, stop, fuse} {
		mount.AddCommand(sub)
	}
	return mount
}
