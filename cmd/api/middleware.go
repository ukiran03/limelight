package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// deferred function, which will always be run in the event of a panic.
		defer func() {
			pv := recover()
			if pv != nil { // there was a panic
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%v", pv))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	if !app.config.limiter.enabled {
		return next
	}

	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu         sync.Mutex
		clientsMap = make(map[string]*client)
	)

	go func() {
		for {
			time.Sleep(time.Minute)

			mu.Lock()
			for ip, client := range clientsMap {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clientsMap, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realip.FromRequest(r) // get the client's IP address.

		mu.Lock()

		if _, found := clientsMap[ip]; !found {
			clientsMap[ip] = &client{
				limiter: rate.NewLimiter(
					rate.Limit(app.config.limiter.rps), // r
					app.config.limiter.burst),          // b
				lastSeen: time.Now(),
			}
		}

		if !clientsMap[ip].limiter.Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w, r)
			return
		}
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
