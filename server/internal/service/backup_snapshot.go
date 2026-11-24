package service

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var backupIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// SnapshotBackupMetadata writes PostgreSQL metadata into the existing portable
// SQLite backup format. PostgreSQL is read under repeatable-read;
// no live metadata.sqlite file is assumed. Native source credentials and the
// instance key are never included. Restore still runs the owner migrations.
func (s *Service) SnapshotBackupMetadata(dir string) error {
	if s.DB.Dialector.Name() != "postgres" {
		return code("backup_backend_unsupported")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.MetadataQuiescence(); err != nil {
		return err
	}
	path := filepath.Join(dir, "metadata.sqlite")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return code("backup_destination_unavailable")
	}
	complete, identityCreated := false, false
	defer func() {
		if !complete {
			for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
				_ = os.Remove(path + suffix)
			}
			if identityCreated {
				_ = os.Remove(filepath.Join(dir, "instance.id"))
			}
		}
	}()
	if err := file.Close(); err != nil {
		return err
	}
	target, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return code("backup_metadata_unavailable")
	}
	connection, err := target.DB()
	if err != nil {
		return code("backup_metadata_unavailable")
	}
	defer connection.Close()
	opts := &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	err = s.DB.Transaction(func(source *gorm.DB) error {
		tables, err := source.Migrator().GetTables()
		if err != nil {
			return err
		}
		return target.Transaction(func(destination *gorm.DB) error {
			for _, table := range tables {
				if table == "sqlite_sequence" {
					continue
				}
				if !backupIdentifier.MatchString(table) {
					return code("backup_schema_unsupported")
				}
				columns, err := source.Migrator().ColumnTypes(table)
				if err != nil {
					return err
				}
				fields := make([]reflect.StructField, 0, len(columns))
				for index, column := range columns {
					if !backupIdentifier.MatchString(column.Name()) {
						return code("backup_schema_unsupported")
					}
					typeOf, err := backupColumnType(column.DatabaseTypeName())
					if err != nil {
						return err
					}
					tag := "column:" + column.Name()
					if primary, _ := column.PrimaryKey(); primary {
						tag += ";primaryKey;autoIncrement:false"
					}
					fields = append(fields, reflect.StructField{Name: fmt.Sprintf("Column%d", index), Type: reflect.PointerTo(typeOf), Tag: reflect.StructTag(`gorm:"` + tag + `"`)})
				}
				rowType := reflect.StructOf(fields)
				if err := destination.Table(table).AutoMigrate(reflect.New(rowType).Interface()); err != nil {
					return err
				}
				if err := copyBackupRows(source, destination, table, rowType); err != nil {
					return err
				}
			}
			return nil
		})
	}, opts)
	if err != nil {
		return code("backup_metadata_unavailable")
	}
	if err := target.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return code("backup_metadata_unavailable")
	}
	if err := connection.Close(); err != nil {
		return code("backup_metadata_unavailable")
	}
	metadata, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return code("backup_metadata_unavailable")
	}
	err = metadata.Sync()
	closeErr := metadata.Close()
	if err != nil || closeErr != nil {
		return code("backup_metadata_unavailable")
	}
	identity, err := os.OpenFile(filepath.Join(dir, "instance.id"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return code("backup_destination_unavailable")
	}
	identityCreated = true
	defer identity.Close()
	if _, err = identity.WriteString(s.Instance); err != nil {
		return err
	}
	if err := identity.Sync(); err != nil {
		return err
	}
	if err := identity.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

// Schema introspection is confined to this maintenance adapter. Column values
// still move through GORM, including sealed byte arrays and nullable fields.
func backupColumnType(name string) (reflect.Type, error) {
	switch strings.ToLower(name) {
	case "integer", "int", "int2", "int4", "int8", "smallint", "bigint":
		return reflect.TypeOf(int64(0)), nil
	case "boolean", "bool":
		return reflect.TypeOf(false), nil
	case "real", "float4", "float8", "double precision":
		return reflect.TypeOf(float64(0)), nil
	case "text", "varchar", "character varying", "char", "uuid":
		return reflect.TypeOf(""), nil
	case "bytea", "blob":
		return reflect.TypeOf([]byte{}), nil
	case "datetime", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone", "date":
		return reflect.TypeOf(time.Time{}), nil
	default:
		return nil, code("backup_schema_unsupported")
	}
}

func copyBackupRows(source, target *gorm.DB, table string, rowType reflect.Type) error {
	rows, err := source.Table(table).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	batch := reflect.MakeSlice(reflect.SliceOf(rowType), 0, 100)
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		if err := target.Table(table).CreateInBatches(batch.Interface(), 100).Error; err != nil {
			return err
		}
		batch = reflect.MakeSlice(reflect.SliceOf(rowType), 0, 100)
		return nil
	}
	var copied int64
	for rows.Next() {
		row := reflect.New(rowType)
		if err := source.ScanRows(rows, row.Interface()); err != nil {
			return err
		}
		batch = reflect.Append(batch, row.Elem())
		copied++
		if batch.Len() == 100 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	var stored int64
	if err := target.Table(table).Count(&stored).Error; err != nil {
		return err
	}
	if copied != stored {
		return code("backup_metadata_mismatch")
	}
	return nil
}
