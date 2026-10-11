// drivebridge-demo-resume upgrades an explicit, checkpointed local demo without
// resetting accounts, metadata or uploaded objects. Never use for production.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
)

func main() {
	state := flag.String("state", "", "Checkpoint state directory (never the active source)")
	listen := flag.String("listen", "127.0.0.1:31949", "Loopback development listener")
	web := flag.String("web-dir", "", "Built client directory")
	user := flag.String("bootstrap-user", "", "Existing demo user ID to appoint as platform administrator")
	flag.Parse()
	if *state == "" || *user == "" {
		fmt.Fprintln(os.Stderr, "state and bootstrap-user required")
		os.Exit(1)
	}
	var ready atomic.Bool
	f, err := fixtures.StartWithOptions(*state, *web, *listen, fixtures.Options{Ready: &ready, RequireExplicitAdminRead: true, Demo: true, ResumeDemo: true, KeepStorage: true, MaxBytes: 1 << 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Checkpoint cannot be resumed")
		os.Exit(1)
	}
	defer f.Close()
	// Only appoint the unique existing local demo principal, never a guessed ID.
	var users []service.User
	if f.Service.DB.Where("name = ?", "本地体验").Find(&users).Error != nil || len(users) != 1 || users[0].ID != *user {
		fmt.Fprintln(os.Stderr, "Demo identity mismatch")
		os.Exit(1)
	}
	store, err := fixtures.DiskStore(filepath.Join(*state, "objects"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Object store unavailable")
		os.Exit(1)
	}
	storage := httptest.NewServer(store)
	defer storage.Close()
	cfg := service.S3Config{Endpoint: storage.URL, Region: "us-east-1", Bucket: "fixture", Prefix: "managed", AccessKey: "fixture-key", SecretKey: "fixture-secret"}
	_, existing := os.Stat(filepath.Join(*state, "objects.ready"))
	count, err := f.Service.CloneDevelopmentPlatform(context.Background(), cfg, existing == nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Object checkpoint verification failed")
		os.Exit(1)
	}
	if err = os.WriteFile(filepath.Join(*state, "objects.ready"), []byte("verified\n"), 0600); err != nil {
		fmt.Fprintln(os.Stderr, "Checkpoint marker failed")
		os.Exit(1)
	}
	if err = f.Service.BootstrapGovernance(*user); err != nil {
		fmt.Fprintln(os.Stderr, "Governance upgrade failed")
		os.Exit(1)
	}
	ready.Store(true)
	fmt.Printf("Demo upgraded; verified %d versions. Listening at %s\n", count, *listen)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	<-done
}
