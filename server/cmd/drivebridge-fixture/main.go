package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"os"
	"os/signal"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8787", "Disposable fixture listener")
	web := flag.String("web-dir", "", "Built web client")
	s3listen := flag.String("s3-listen", "", "Expose the fixture S3 on this loopback address (offline rehearsal only)")
	governance := flag.Bool("governance", false, "Enable disposable multi-tenant governance fixtures")
	failArchive := flag.Bool("fail-archive-publish-once", false, "Fail the first archive publication transaction (disposable loopback recovery test only)")
	transferPlans := flag.Bool("transfer-plans", false, "Enable transfer plans in disposable fixtures")
	propertyEditing := flag.Bool("property-editing", false, "Enable binary sidecar property edits in disposable fixtures")
	automationManagement := flag.Bool("automation-management", false, "Enable human-confirmed automation in disposable fixtures")
	shareReceivers := flag.Bool("share-receivers", false, "Enable share receiver proof and device flows in disposable fixtures")
	holdManagement := flag.Bool("hold-management", false, "Enable safe human retention hold management in disposable fixtures")
	labelManagement := flag.Bool("label-management", false, "Enable human label dictionary and selected-file merge in disposable fixtures")
	demo := flag.Bool("demo", false, "Enable one-click access to disposable local sample data")
	publicOrigin := flag.String("public-origin", "", "Exact HTTP loopback browser origin when using port forwarding")
	flag.Parse()
	state, e := os.MkdirTemp("", "drivebridge-fixture-")
	if e != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(state)
	f, e := fixtures.StartWithOptions(state, *web, *listen, fixtures.Options{Demo: *demo, TeamDemo: *demo && *governance, PublicOrigin: *publicOrigin, FailArchivePublishOnce: *failArchive, TransferPlans: *transferPlans, PropertyEditing: *propertyEditing, AutomationManagement: *automationManagement, ShareReceivers: *shareReceivers, HoldManagement: *holdManagement, LabelManagement: *labelManagement})
	if e != nil {
		fmt.Fprintln(os.Stderr, "Fixture startup failed.")
		os.Exit(1)
	}
	defer f.Close()
	if *s3listen != "" {
		host, _, e := net.SplitHostPort(*s3listen)
		if ip := net.ParseIP(host); e != nil || ip == nil || !ip.IsLoopback() {
			fmt.Fprintln(os.Stderr, "Fixture S3 listener must be a numeric loopback address.")
			os.Exit(1)
		}
		s3srv := &http.Server{Addr: *s3listen, Handler: f.S3Handler()}
		s3ln, e := net.Listen("tcp", *s3listen)
		if e != nil {
			fmt.Fprintln(os.Stderr, "Fixture S3 listener failed.")
			os.Exit(1)
		}
		defer s3srv.Close()
		go s3srv.Serve(s3ln)
		fmt.Fprintln(os.Stderr, "Fixture S3 endpoint at http://"+s3ln.Addr().String())
	}
	// E2e-only fixed credentials, same convention as the fixture S3 pair.
	issued, seedErr := f.Service.ImportUsers([]service.UserImport{
		{Name: "e2e-key-user", Key: "dbu_" + strings.Repeat("11", 48)},
		{Name: "e2e-admin", Key: "dbu_" + strings.Repeat("22", 48), Admin: true},
	})
	if seedErr != nil {
		fmt.Fprintln(os.Stderr, "Fixture key seeding failed.")
		os.Exit(1)
	}
	if *governance && !*demo {
		for _, u := range issued {
			if e = f.Service.DB.Create(&service.Space{ID: service.ID(), Owner: u.UserID, Name: u.Name + " personal", ConnectionID: "platform", MaxBytes: 1 << 30}).Error; e != nil {
				os.Exit(1)
			}
		}
		if e = f.Service.BootstrapGovernance(issued[1].UserID); e != nil {
			fmt.Fprintln(os.Stderr, "Fixture governance setup failed.")
			os.Exit(1)
		}
	}
	fmt.Println("Offline fixture ready at " + f.Service.Config.Origin)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	<-done
}
