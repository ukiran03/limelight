package main

import (
	"context"
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ukiran.com/limelight/internal/data"
	"ukiran.com/limelight/internal/mailer"
	"ukiran.com/limelight/internal/vcs"
)

var version = vcs.Version()

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	mailer *mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development",
		"Environment (development|staging|production)")
	flag.StringVar(
		&cfg.db.dsn,
		"db-dsn",
		os.Getenv("LIMELIGHT_DB_DSN"),
		"PostgreSQL DSN")

	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns",
		25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns",
		25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time",
		15*time.Minute, "PostgreSQL max connection idle time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps",
		2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst",
		4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled",
		true, "Enable rate limiter")

	// Read the SMTP server configuration settings into the config struct,
	// using the Mailtrap settings as the default values. IMPORTANT: If you're
	// following along, make sure to replace the default values for
	// smtp-username and smtp-password with your own Mailtrap credentials.

	flag.StringVar(&cfg.smtp.host, "smtp-host",
		"sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username",
		"17f17f001f2b81", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password",
		"e99508951400a1", "SMTP password")
	flag.StringVar(
		&cfg.smtp.sender, "smtp-sender",
		"Limelight <no-reply@limelight.ukiran.com>", "SMTP sender")

	// Use the flag.Func() function to process the -cors-trusted-origins
	// command line flag. In this we use the strings.Fields() function to split
	// the flag value into a slice based on whitespace characters and assign it
	// to our config struct.  Importantly, if the -cors-trusted-origins flag is
	// not present, contains the empty string, or contains only whitespace,
	// then strings.Fields() will return an empty []string slice.
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)",
		func(val string) error {
			cfg.cors.trustedOrigins = strings.Fields(val)
			return nil
		},
	)
	// Create a new version boolean flag with the default value of false.
	displayVersion := flag.Bool("version", false, "Display version and exit")
	flag.Parse()
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	logger := NewLogger()

	db, err := openDB(cfg)
	if err != nil {
		logger.Error("unable to connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("database connection pool established")

	mailer, err := mailer.New(
		cfg.smtp.host, cfg.smtp.port, cfg.smtp.username,
		cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	expvar.NewString("version").Set(version)
	// Publish the number of active goroutines.
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	// Publish the database connection pool statistics.
	expvar.Publish("database", expvar.Func(func() any {
		return NewPgxDBStats(db.Stat())
	}))

	// Publish the current Unix timestamp.
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer,
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	poolCfg.MaxConns = int32(cfg.db.maxOpenConns)
	poolCfg.MinConns = int32(cfg.db.maxIdleConns)
	poolCfg.MaxConnIdleTime = cfg.db.maxIdleTime
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	// pgxpool.NewWithConfig is "lazy". It doesn't connect until used.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, err
}
