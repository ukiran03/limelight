package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve(cacheStop func()) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	// this will be used to recieve any errors returned by the graceful
	// Shutdown() function
	shutdownErrChan := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)

		// os.Interrupt handles SIGINT (Ctrl+C)
		// syscall.SIGTERM handles orchestration shutdowns
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		// read the signal from the quit chan, this will block until a
		// signal is received.
		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop the HTTP server first: No new HTTP requests can come in, and
		// active requests have 30s to finish.
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownErrChan <- err
		}

		// Now that the API is shut down and no new cache jobs can possibly be
		// sent, safely stop the workers and drain the remaining queue items.
		app.logger.Info("completing redis cache jobs")
		cacheStop()

		app.logger.Info("completing background tasks", "addr", srv.Addr)
		// Wait for any other general application background tasks
		app.wg.Wait()          // block until counter is zero
		shutdownErrChan <- nil // then complete the shutdown
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	err = <-shutdownErrChan
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil
}
