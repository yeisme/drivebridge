package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/internal/config"
	"github.com/yeisme/drivebridge-server/internal/service"
)

// cli carries the resolved configuration shared by every subcommand.
type cli struct {
	root                  *cobra.Command
	command               string
	mode                  outputMode
	configPath, flagState string
	effective             config.Effective
	// teamDSN carries the team PostgreSQL metadata DSN from --team-dsn or
	// DRIVEBRIDGE_TEAM_DSN. It bypasses the resolved config so it can never
	// appear in config output, logs, or receipts.
	teamDSN string
}

// flagNames maps configuration keys to their command-line flag names.
var flagNames = map[string]string{
	"service_identity_issuer":   "service-identity-issuer",
	"service_identity_endpoint": "service-identity-endpoint",
	"state":                     "state",
	"listen":                    "listen",
	"origin":                    "origin",
	"issuer":                    "issuer",
	"client_id":                 "client-id",
	"client_secret_file":        "client-secret-file",
	"onedrive_client_id":        "onedrive-client-id",
	"onedrive_secret_file":      "onedrive-secret-file",
	"web_dir":                   "web-dir",
	"max_bytes":                 "max-bytes",
	"tenant_name":               "tenant-name",
	"tenant_origin":             "tenant-origin",
	"tenant_contact":            "tenant-contact",
	"document_editing":          "document-editing",
	"property_editing":          "property-editing",
	"automation_management":     "automation-management",
	"share_receivers":           "share-receivers",
	"hold_management":           "hold-management",
	"label_management":          "label-management",
}

// resolve loads the optional config file and resolves every known key.
// config init tolerates a missing explicit path: that file is its target.
func (c *cli) resolve(cmd *cobra.Command) error {
	path := config.Discover(c.configPath)
	tolerateMissing := cmd.Parent() != nil && cmd.Parent().Name() == "config" && cmd.Name() == "init"
	var file config.File
	if path != "" {
		loaded, err := config.Load(path)
		if err != nil {
			if !(tolerateMissing && errors.Is(err, fs.ErrNotExist)) {
				return err
			}
		} else {
			file = loaded
		}
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	flags := map[string]config.Flag{}
	for key, name := range flagNames {
		if cmd.Flags().Lookup(name) != nil && cmd.Flags().Changed(name) {
			flags[key] = config.Flag{Value: cmd.Flags().Lookup(name).Value.String(), Set: true}
		}
	}
	c.effective = config.ResolveAll(file, flags, map[string]string{
		"state":     filepath.Join(home, "drivebridge-server"),
		"listen":    "127.0.0.1:8787",
		"max_bytes": "1073741824",
	})
	return nil
}

// serviceConfig materializes the runtime config; enabled and development are
// flag-only by contract and never come from files or the environment.
func (c *cli) serviceConfig(enabled, development bool) (service.Config, error) {
	cfg := service.Config{
		ServiceIdentityIssuer:   c.effective.Values["service_identity_issuer"],
		ServiceIdentityEndpoint: c.effective.Values["service_identity_endpoint"],
		State:                   c.effective.Values["state"],
		Origin:                  c.effective.Values["origin"],
		Issuer:                  c.effective.Values["issuer"],
		ClientID:                c.effective.Values["client_id"],
		MicrosoftClientID:       c.effective.Values["onedrive_client_id"],
		TenantName:              c.effective.Values["tenant_name"],
		TenantOrigin:            c.effective.Values["tenant_origin"],
		TenantContact:           c.effective.Values["tenant_contact"],
		Enabled:                 enabled,
		Development:             development,
		MaxBytes:                c.effective.Int("max_bytes", 1<<30),
		WebDir:                  c.effective.Values["web_dir"],
		TeamDSN:                 c.teamDSN,
		DocumentEditing:         c.effective.Values["document_editing"] == "true",
		PropertyEditing:         c.effective.Values["property_editing"] == "true",
		AutomationManagement:    c.effective.Values["automation_management"] == "true",
		ShareReceivers:          c.effective.Values["share_receivers"] == "true",
		HoldManagement:          c.effective.Values["hold_management"] == "true",
		LabelManagement:         c.effective.Values["label_management"] == "true",
	}
	secret, err := readSecret(c.effective.Values["client_secret_file"])
	if err != nil {
		return cfg, err
	}
	cfg.ClientSecret = secret
	msecret, err := readSecret(c.effective.Values["onedrive_secret_file"])
	if err != nil {
		return cfg, err
	}
	cfg.MicrosoftSecret = msecret
	reporter, err := readSecret(c.effective.Values["service_access_reporter_credential_file"])
	if err != nil {
		return cfg, err
	}
	cfg.ServiceAdmissionCredential = reporter
	return cfg, nil
}
