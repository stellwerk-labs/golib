package hpostgres

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/stellwerk-labs/golib/hpostgresconnect"
	"go.uber.org/zap"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // File source driver for migrations
)

type Config = hpostgresconnect.Config

// Model is the underlying type for the entire model.
type Database struct {
	hpostgresconnect.Database
	logger *zap.Logger
}

// Migrate applies database migrations
func (db *Database) Migrate(ctx context.Context, migrationsFolder string) error {
	db.logger.Info("Running migrations")
	var m *migrate.Migrate

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err = migrate.NewWithDatabaseInstance("file://"+migrationsFolder, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}

		if errors.Is(err, os.ErrNotExist) {
			// ignore if current migration doesn't exist, could be a result of the code rollback
			db.logger.Sugar().Warnf("WARNING: %v", err)
			return nil
		}

		return err
	}
	return nil
}

// InitDatabase initialize database
func InitDatabase(ctx context.Context, config *Config) (*Database, error) {
	db, err := hpostgresconnect.InitDatabase(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Database{Database: *db, logger: config.Logger}, nil
}
