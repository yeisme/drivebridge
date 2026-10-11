package main

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/internal/service"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newMetadataCommand wires the operator migration surface onto the same
// application service the HTTP runtime uses. No credential is ever a command
// parameter: the PostgreSQL target DSN comes from DRIVEBRIDGE_TEAM_DSN, and
// the sqlite target is an explicit empty state directory path.
func newMetadataCommand(c *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Quiesced metadata export/import/verify for SQLite to PostgreSQL migration.",
	}
	var outFile string
	export := &cobra.Command{
		Use:   "export",
		Short: "Export the quiesced metadata snapshot as JSONL (no object bytes, no instance key).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if outFile == "" {
				return fail("output_required", "Pass --out with the snapshot file path.")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			file, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return fail("output_unwritable", "The snapshot file could not be created with 0600.")
			}
			defer file.Close()
			if err := s.ExportMetadata(file); err != nil {
				return err
			}
			return c.mode.emitData("metadata export", "success", "Quiesced metadata snapshot written; object bytes and the instance key are not included.", map[string]any{"output": outFile, "instance_id": s.Instance})
		},
	}
	export.Flags().StringVar(&outFile, "out", "", "Snapshot file path (created 0600)")
	var sourceFile, targetState string
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import a snapshot into a fresh target database (sqlite state dir, or PostgreSQL via DRIVEBRIDGE_TEAM_DSN).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if sourceFile == "" {
				return fail("source_required", "Pass --source with the snapshot file path.")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			file, err := os.Open(sourceFile)
			if err != nil {
				return fail("source_unreadable", "The snapshot file could not be opened.")
			}
			defer file.Close()
			target, err := openMigrationTarget(targetState)
			if err != nil {
				return err
			}
			result, err := s.ImportMetadata(file, target)
			if err != nil {
				return err
			}
			return c.mode.emitData("metadata import", "success", "Snapshot imported into the target; verify before switching writes.", result)
		},
	}
	importCmd.Flags().StringVar(&sourceFile, "source", "", "Snapshot file path")
	importCmd.Flags().StringVar(&targetState, "target-state", "", "Empty target state directory (sqlite); omit to use DRIVEBRIDGE_TEAM_DSN")
	var leftFile, rightFile string
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Compare two metadata snapshots (instance identity, table counts).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if leftFile == "" || rightFile == "" {
				return fail("snapshots_required", "Pass --left and --right snapshot files.")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			left, err := os.Open(leftFile)
			if err != nil {
				return fail("source_unreadable", "The left snapshot could not be opened.")
			}
			defer left.Close()
			right, err := os.Open(rightFile)
			if err != nil {
				return fail("source_unreadable", "The right snapshot could not be opened.")
			}
			defer right.Close()
			result, err := s.VerifyMetadata(left, right)
			if err != nil {
				return err
			}
			if m, ok := result["matching"].(bool); ok && !m {
				return fail("metadata_mismatch", "Snapshots differ; do not switch writes.")
			}
			return c.mode.emitData("metadata verify", "success", "Snapshots match on instance identity and table counts.", result)
		},
	}
	verify.Flags().StringVar(&leftFile, "left", "", "Left snapshot file path")
	verify.Flags().StringVar(&rightFile, "right", "", "Right snapshot file path")
	downgrade := &cobra.Command{
		Use:   "check-workspace-downgrade",
		Short: "After stopping issuance and workers, reject downgrade while any workspace grant is not revoked.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.CheckWorkspaceDowngrade(); err != nil {
				return err
			}
			return c.mode.emitData("metadata check-workspace-downgrade", "success", "No unrevoked workspace grants. This does not certify other schemas or authorize a downgrade; keep issuance and workers stopped.", map[string]any{"workspace_grants_retired": true})
		},
	}
	cmd.AddCommand(export, importCmd, verify, downgrade)
	return cmd
}

// openMigrationTarget opens the import target. A target DSN never appears in
// flags, arguments, or output; it is read from the environment only.
func openMigrationTarget(stateDir string) (*gorm.DB, error) {
	if dsn := os.Getenv("DRIVEBRIDGE_TEAM_DSN"); dsn != "" && stateDir == "" {
		db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			return nil, fail("target_unavailable", "The PostgreSQL target refused the connection; check DRIVEBRIDGE_TEAM_DSN.")
		}
		if err := db.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{}); err != nil {
			return nil, fail("target_unavailable", "The PostgreSQL target schema could not be prepared.")
		}
		return db, nil
	}
	if stateDir == "" {
		return nil, fail("target_required", "Pass --target-state with an empty state directory, or set DRIVEBRIDGE_TEAM_DSN.")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fail("target_unavailable", "The target state directory could not be created.")
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(stateDir, "metadata.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fail("target_unavailable", "The sqlite target could not be opened.")
	}
	if err := db.AutoMigrate(&service.Space{}, &service.Connection{}, &service.File{}, &service.Version{}, &service.Operation{}, &service.AccessGrant{}); err != nil {
		return nil, fail("target_unavailable", "The sqlite target schema could not be prepared.")
	}
	return db, nil
}
