package main

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
)

// Instance events are available only to an operator holding the private state
// directory. They are never served through a normal user's HTTP identity.
func newInstanceAuditCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "audit", Short: "Read instance-only security events from private local state."}
	for _, action := range []string{"list", "export"} {
		action := action
		var instance bool
		var from, to string
		cmd := &cobra.Command{Use: action, RunE: func(cmd *cobra.Command, _ []string) error {
			if !instance {
				return fmt.Errorf("instance_scope_required")
			}
			state := c.effective.Values["state"]
			info, e := os.Stat(state)
			if e != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
				return fmt.Errorf("state_not_private")
			}
			file := filepath.Join(state, "metadata.sqlite")
			if _, e = os.Stat(file); e != nil {
				return fmt.Errorf("state_unavailable")
			}
			u := url.URL{Scheme: "file", Path: file, RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
			db, e := gorm.Open(sqlite.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if e != nil {
				return fmt.Errorf("state_unavailable")
			}
			raw, _ := db.DB()
			defer raw.Close()
			out, e := insights.ReadOnly(db).Audit(insights.Scope{Owner: ""}, insights.AuditFilter{From: from, To: to, Limit: 200})
			if e != nil {
				return e
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				for _, event := range out.Events {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s  %s\n", event.At.Format("2006-01-02T15:04:05Z"), event.Action, event.Actor, event.Outcome, event.Resource)
				}
			}
			return c.mode.emitData("audit."+action, "success", fmt.Sprintf("%d instance event(s); user file activity excluded.", len(out.Events)), map[string]any{"events": out.Events, "next_cursor": out.Next, "scope": "instance", "count": len(out.Events)})
		}}
		cmd.Flags().BoolVar(&instance, "instance", false, "Explicitly select operator-only instance security events")
		cmd.Flags().StringVar(&from, "from", "", "First UTC date YYYY-MM-DD")
		cmd.Flags().StringVar(&to, "to", "", "Last UTC date YYYY-MM-DD")
		root.AddCommand(cmd)
	}
	return root
}
