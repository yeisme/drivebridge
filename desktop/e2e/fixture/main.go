// Command e2e-fixture prepares a disposable owner state for the desktop
// end-to-end suite. It is test infrastructure: it registers one space per
// <vault:space> pair and fences the state through the owner protocol so guard
// admission has a provable layout. It must never point at a real deployment
// state or a user vault.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"github.com/yeisme/drivebridge/internal/bridge"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: e2e-fixture <state> <vault>:<space> [...]")
		os.Exit(2)
	}
	state := os.Args[1]
	type registration struct{ vault, space string }
	var registrations []registration
	for _, pair := range os.Args[2:] {
		vault, space, ok := strings.Cut(pair, ":")
		if !ok || vault == "" || space == "" {
			fmt.Fprintln(os.Stderr, "invalid <vault>:<space> pair")
			os.Exit(2)
		}
		registrations = append(registrations, registration{vault: vault, space: space})
	}
	service, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "state:", err)
		os.Exit(1)
	}
	for _, item := range registrations {
		if err = os.MkdirAll(item.vault, 0700); err != nil {
			service.Close()
			fmt.Fprintln(os.Stderr, "vault:", err)
			os.Exit(1)
		}
		if err = service.DB.Create(&bridge.Space{ID: item.space, Remote: "remote", LocalRoot: item.vault, MaxBytes: 1 << 30}).Error; err != nil {
			service.Close()
			fmt.Fprintln(os.Stderr, "space:", err)
			os.Exit(1)
		}
		if err = service.DB.Create(&bridge.AdoptRecord{ID: "owner-" + item.space, Consumer: "pinax", Kind: "local", SpaceID: item.space, Location: item.vault, IdempotencyKey: "owner-" + item.space}).Error; err != nil {
			service.Close()
			fmt.Fprintln(os.Stderr, "adopt:", err)
			os.Exit(1)
		}
	}
	service.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		fmt.Fprintln(os.Stderr, "lock:", err)
		os.Exit(1)
	}
	plan, err := managedstate.Prepare(state, "cli", "migration-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, "prepare:", err)
		os.Exit(1)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		os.Exit(1)
	}
	fmt.Println("ok")
}
