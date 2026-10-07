// Command migrate applies the database migrations embedded in the binary.
//
// It is a separate program from the API on purpose: in production it runs
// once per deploy as a Cloud Run job, before the new API revision receives
// traffic. If migrations ran on API startup instead, every instance would
// race to migrate, and a failed migration would crash-loop the API.
//
// Usage:
//
//	DATABASE_URL=postgresql://... migrate up      # apply all pending migrations
//	DATABASE_URL=postgresql://... migrate down    # roll back the latest one
//	DATABASE_URL=postgresql://... migrate status  # list applied and pending
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5"
	// The pgx driver for database/sql, which goose needs.
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/migrations"
)

const usage = "usage: migrate <up|down|status>"

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		// The configured log level is unknown here, so use the default one.
		logging.New(os.Stdout, config.DefaultLogLevel).Error("cannot start", "error", err)
		os.Exit(2)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, logging.WithService("meurpg-migrate"))

	if err := run(logger, cfg, os.Args[1:]); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg config.Config, args []string) error {
	if len(args) != 1 {
		return errors.New(usage)
	}
	command := args[0]
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	// Ctrl+C or SIGTERM cancels the context; goose then stops between
	// statements. What is left is NOT rolled back for DDL: CockroachDB commits
	// the open transaction before each schema change (autocommit_before_ddl), so
	// a migration with several statements can stop halfway, and goose has not
	// recorded it. Every migration is written to run again from the start
	// (TestMigrationsAreSafeToRerun), so running `migrate up` again finishes it.
	// See the package comment of backend/migrations.
	//
	// `up` and `down` take a lease lock first (migrations.AcquireLock), because
	// goose has none for CockroachDB: a second run waits for the first.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Parse the URL ourselves: an error from sql.Open("pgx", url) can echo the
	// connection string, which holds the password. The error here is a fixed
	// message, as in platform/db.
	connConfig, err := pgx.ParseConfig(cfg.DatabaseURL.Reveal())
	if err != nil {
		return errors.New("DATABASE_URL is not a valid PostgreSQL connection string")
	}
	db := stdlib.OpenDB(*connConfig)
	defer func() { _ = db.Close() }()

	provider, err := migrations.NewProvider(db)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	if command == "up" || command == "down" {
		runCtx, release, err := migrations.AcquireLock(ctx, db, migrations.LockOptions{Logger: logger})
		if err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		defer release()
		ctx = runCtx
	}

	switch command {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("up: %w", lockCause(ctx, err))
		}
		for _, r := range results {
			logger.Info("migration applied", "version", r.Source.Version, "file", r.Source.Path, "duration_ms", r.Duration.Milliseconds())
		}
		logger.Info("database is up to date", "applied", len(results))
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return fmt.Errorf("down: %w", lockCause(ctx, err))
		}
		logger.Info("migration rolled back", "version", result.Source.Version, "file", result.Source.Path)
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		for _, s := range statuses {
			attrs := []any{"version", s.Source.Version, "file", s.Source.Path, "state", string(s.State)}
			if !s.AppliedAt.IsZero() {
				attrs = append(attrs, "applied_at", s.AppliedAt)
			}
			logger.Info("migration", attrs...)
		}
	default:
		return fmt.Errorf("unknown command %q; %s", command, usage)
	}
	return nil
}

// lockCause says why a run stopped when the lock code canceled it (the lease was
// lost), instead of goose's bare "context canceled".
func lockCause(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		return fmt.Errorf("%w (%w)", err, cause)
	}
	return err
}
