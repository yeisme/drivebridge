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
			body, err := os.ReadFile(filepath.Join(c.effective.Values["state"], "mount.json"))
			if err != nil {
				return c.mode.write("mount.status", "success", "No mount is running.", map[string]any{"alive": false})
			}
			var st bridge.MountState
			if json.Unmarshal(body, &st) != nil {
				return fail("state_unavailable", "The mount receipt is unreadable.")
			}
			alive := false
			if conn, err := net.DialTimeout("tcp", st.Listen, 200*time.Millisecond); err == nil {
				conn.Close()
				alive = true
			}
			return c.mode.write("mount.status", "success", fmt.Sprintf("Mount %s at %s (alive=%v).", st.Adapter, st.Listen, alive), map[string]any{"alive": alive, "adapter": st.Adapter, "listen": st.Listen, "mode": st.Mode, "space_ids": st.SpaceIDs, "started_at": st.StartedAt})
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

	for _, sub := range []*cobra.Command{webdav, status, stop, fuse} {
		mount.AddCommand(sub)
	}
	return mount
}
