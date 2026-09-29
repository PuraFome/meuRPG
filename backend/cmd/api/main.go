// Command api is MeuRPG's HTTP server: it serves the Connect RPCs used by the
// web app plus the /healthz and /readyz probes.
//
// Configuration comes from the environment (see internal/platform/config):
//
//	PORT          listen port (default 8080)
//	DATABASE_URL  CockroachDB connection string (optional)
//	LOG_LEVEL     debug, info, warn or error (default info)
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/httpserver"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/system"
)

// Build information, replaced at build time with:
//
//	go build -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse HEAD)"
var (
	version = "dev"
	commit  = "unknown"
)

// maxRequestBytes caps the size of a single RPC message the server will
// read, so a client cannot exhaust memory with one huge request.
const maxRequestBytes = 4 << 20 // 4 MiB

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		// The configured log level is unknown here, so use the default one.
		logging.New(os.Stdout, config.DefaultLogLevel).Error("cannot start", "error", err)
		os.Exit(2)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel)

	if err := run(logger, cfg); err != nil {
		logger.Error("api stopped with an error", "error", err)
		os.Exit(1)
	}
}

// run wires the application together. Keeping it apart from main means
// deferred cleanups (closing the pool) run before the process exits.
func run(logger *slog.Logger, cfg config.Config) error {
	// Cancel ctx on Ctrl+C (SIGINT) or SIGTERM, which is how Cloud Run asks
	// a container to stop. Canceling ctx starts the graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore the default behavior so a second
	// Ctrl+C kills the process right away instead of waiting for the drain.
	context.AfterFunc(ctx, stop)

	logger.Info("starting api", "version", version, "commit", commit)

	// database stays a nil interface when DATABASE_URL is empty. It must not
	// hold a nil *pgxpool.Pool: an interface holding a typed nil pointer is
	// not == nil, and /readyz would then call Ping on it.
	var database httpserver.Pinger
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL is not set; running without a database")
	} else {
		pool, err := db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		// A failed ping is not fatal: the database may come up later, and
		// until it does /readyz reports it as down.
		if err := pool.Ping(ctx); err != nil {
			logger.Warn("database is not reachable yet", "error", err)
		}
		database = pool
	}

	srv := httpserver.New(httpserver.Config{
		Addr:   ":" + strconv.Itoa(cfg.Port),
		Logger: logger,
		DB:     database,
	})

	connectOpts := []connect.HandlerOption{
		connect.WithReadMaxBytes(maxRequestBytes),
	}
	srv.Handle(systemv1connect.NewSystemServiceHandler(system.NewService(version, commit), connectOpts...))

	if static, ok := httpserver.NewStatic(cfg.WebDir); ok {
		srv.Handle("/", static)
		logger.Info("serving the web app", "dir", cfg.WebDir)
	} else {
		logger.Info("no web build found; running API-only", "dir", cfg.WebDir)
	}

	return srv.Run(ctx)
}
