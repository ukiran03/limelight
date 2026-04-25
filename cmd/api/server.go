package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	go func() {
		quit := make(chan os.Signal, 1)

		// os.Interrupt handles SIGINT (Ctrl+C)
		// syscall.SIGTERM handles orchestration shutdowns
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		// read the signal from the quit chan, this will block until a
		// signal is received.
		s := <-quit

		app.logger.Info("caught signal", "signal", s.String())
		os.Exit(0)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	return srv.ListenAndServe()
}
