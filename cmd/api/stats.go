package main

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxDBStats maps pgxpool.Stat metrics to the naming convention
// used by the standard library's database/sql.DBStats.
type PgxDBStats struct {
	// Pool Status
	MaxOpenConnections int // Map: MaxConns()
	OpenConnections    int // Map: TotalConns()
	InUse              int // Map: AcquiredConns()
	Idle               int // Map: IdleConns()

	// Counters
	WaitCount         int64         // Map: EmptyAcquireCount()
	WaitDuration      time.Duration // Map: EmptyAcquireWaitTime()
	MaxIdleClosed     int64         // Map: MaxIdleDestroyCount()
	MaxLifetimeClosed int64         // Map: MaxLifetimeDestroyCount()

	// pgx Specific (Extra Context)
	CanceledAcquireCount int64
	ConstructingConns    int
}

func NewPgxDBStats(s *pgxpool.Stat) PgxDBStats {
	return PgxDBStats{
		// Pool Status
		MaxOpenConnections: int(s.MaxConns()),      // The limit
		OpenConnections:    int(s.TotalConns()),    // Currently established (Idle + InUse + Constructing)
		InUse:              int(s.AcquiredConns()), // Actively doing work
		Idle:               int(s.IdleConns()),     // Sitting ready

		// Counters
		WaitCount:         s.EmptyAcquireCount(),    // Times workers had to wait for a connection
		WaitDuration:      s.EmptyAcquireWaitTime(), // Total time spent waiting (Duration)
		MaxIdleClosed:     s.MaxIdleDestroyCount(),
		MaxLifetimeClosed: s.MaxLifetimeDestroyCount(),

		// pgx Specific
		CanceledAcquireCount: s.CanceledAcquireCount(),
		ConstructingConns:    int(s.ConstructingConns()),
	}
}
