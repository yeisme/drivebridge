package bridge

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

// MountState is the CLI-owned mount receipt; the MCP mount_status tool reads
// it read-only and never carries authority to start or stop a mount.
type MountState struct {
	Adapter   string    `json:"adapter"`
	Listen    string    `json:"listen"`
	Mode      string    `json:"mode"`
	SpaceIDs  []string  `json:"space_ids"`
	Token     string    `json:"token"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func (s *Service) mountStatePath() string { return filepath.Join(s.state, "mount.json") }

// RegisterMount claims the single per-state mount slot; a live or stale
// receipt blocks a second mount until the owner stops it.
func (s *Service) RegisterMount(st MountState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return errors.New("invalid_mount")
	}
	f, err := os.OpenFile(s.mountStatePath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("mount_already_running")
	}
	defer f.Close()
	if _, err = f.Write(body); err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// ClearMount releases the slot only for the owning process receipt.
func (s *Service) ClearMount(pid int) {
	st, ok := s.MountReceipt()
	if !ok || st.PID != pid {
		return
	}
	_ = os.Remove(s.mountStatePath())
}

// MountReceipt reads the raw CLI-written receipt without a liveness claim.
func (s *Service) MountReceipt() (MountState, bool) {
	var st MountState
	body, err := os.ReadFile(s.mountStatePath())
	if err != nil || json.Unmarshal(body, &st) != nil || st.Adapter == "" {
		return MountState{}, false
	}
	return st, true
}

// MountStatus is the MCP projection: never includes the session token, and
// alive is a real loopback dial probe rather than a stale-file guess.
func (s *Service) MountStatus() map[string]any {
	st, ok := s.MountReceipt()
	if !ok {
		return map[string]any{"alive": false, "adapter": "", "listen": "", "mode": "", "space_ids": []string{}}
	}
	alive := false
	if st.Listen != "" {
		if conn, err := net.DialTimeout("tcp", st.Listen, 200*time.Millisecond); err == nil {
			conn.Close()
			alive = true
		}
	}
	return map[string]any{"alive": alive, "adapter": st.Adapter, "listen": st.Listen, "mode": st.Mode, "space_ids": st.SpaceIDs, "started_at": st.StartedAt}
}
