package hdbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib" // required by goose
)

const (
	migrationsDir           = "./migrations"
	advisoryLockIDSalt uint = 1486364155
)

func Migrate(ctx context.Context, args []string, dbstring string, log *zap.Logger) error {
	migrationsPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return err
	}

	db, err := goose.OpenDBWithDriver("postgres", dbstring)
	if err != nil {
		return err
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Sugar().Warnw("goose: failed to close DB", "err", err)
		}
	}()

	dbname, err := databaseName(ctx, db)
	if err != nil {
		return err
	}

	aid, err := generateAdvisoryLockId(dbname)
	if err != nil {
		return err
	}

	if err := lock(ctx, aid, db); err != nil {
		return err
	}

	defer func() {
		if err := unlock(ctx, aid, db); err != nil {
			log.Sugar().Warnw("failed to unlock", "err", err)
		}
	}()

	command := args[0]
	arguments := []string{}
	if len(args) > 1 {
		arguments = append(arguments, args[1:]...)
	}

	if err := goose.RunContext(ctx, command, db, migrationsPath, arguments...); err != nil {
		return fmt.Errorf("goose %v: %w", command, err)
	}

	return nil
}

func lock(ctx context.Context, aid string, conn *sql.DB) error {
	// This will wait indefinitely until the lock can be acquired.
	query := `SELECT pg_advisory_lock($1)`
	if _, err := conn.ExecContext(ctx, query, aid); err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	return nil
}

func unlock(ctx context.Context, aid string, conn *sql.DB) error {
	query := `SELECT pg_advisory_unlock($1)`
	if _, err := conn.ExecContext(ctx, query, aid); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}

	return nil
}

// Source https://github.com/golang-migrate/migrate/blob/7cacfc20803a4f22165fb4a444bb88ad5dfba2fa/database/util.go#L13
func generateAdvisoryLockId(databaseName string, additionalNames ...string) (string, error) {
	if len(additionalNames) > 0 {
		databaseName = strings.Join(append(additionalNames, databaseName), "\x00")
	}
	sum := crc32.ChecksumIEEE([]byte(databaseName))
	sum = sum * uint32(advisoryLockIDSalt)
	return fmt.Sprint(sum), nil
}

func databaseName(ctx context.Context, conn *sql.DB) (string, error) {
	row := conn.QueryRowContext(ctx, "SELECT current_database();")
	if row.Err() != nil {
		return "", row.Err()
	}
	var dbname string
	if err := row.Scan(&dbname); err != nil {
		return "", err
	}

	return dbname, nil
}
