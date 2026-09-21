// Package persistence wires Postgres connectivity (read/write split with
// Datadog tracing) and the transaction manager used by application-layer
// use cases.
//
// The Master and Replica connections are typed as
// safesql.MasterDB / safesql.ReplicaDB respectively. The replica interface
// has no write methods, so the type system makes it compile-impossible to
// mutate via the read connection.
package persistence

import (
	"fmt"

	"github.com/astronautsid/astro-boilerplate/internal/config"
	"github.com/astronautsid/astro-golibs/safesql"
)

// NewMasterDB opens a Datadog-traced Postgres master connection. safesql
// internally wraps lib/pq with the dd-trace-go sqlx contrib, exposes
// pool-stat metrics, and returns a write-capable handle whose interface
// also satisfies the read methods (so transactional reads of just-written
// rows work). Callers must Close() at shutdown.
//
// The DSN is injected verbatim from cfg.DSN — credentials and host are not
// reconstructed here. cfg.App.Name (passed as serviceName by main.go) is
// used as a fallback for the Datadog trace name when safesql cannot
// extract a dbname from the DSN.
func NewMasterDB(cfg config.PostgresInstance, serviceName string) (safesql.MasterDB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: empty master DSN for service %q", serviceName)
	}
	return safesql.OpenMasterWithOption("postgres", serviceName, cfg.DSN, toOption(cfg))
}

// NewReplicaDB opens a Datadog-traced Postgres replica connection. The
// returned safesql.ReplicaDB exposes only read methods — attempting to
// call ExecContext, BeginTxx, or any other write API on it is a compile
// error, eliminating an entire class of read/write-split bugs.
func NewReplicaDB(cfg config.PostgresInstance, serviceName string) (safesql.ReplicaDB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: empty replica DSN for service %q", serviceName)
	}
	return safesql.OpenReplicaWithOption("postgres", serviceName, cfg.DSN, toOption(cfg))
}

// toOption converts the boilerplate's PostgresInstance (seconds-based, more
// human-readable in YAML) to safesql's Option (millisecond-based, matches
// the upstream API). safesql treats values <= 0 as "use library default"
// so leaving any field zero in YAML falls through to safesql's behavior.
func toOption(cfg config.PostgresInstance) safesql.Option {
	return safesql.Option{
		MaxConnOpen: cfg.MaxOpen,
		MaxConnIdle: cfg.MaxIdle,
		MaxTtl:      cfg.MaxLifetimeSec * 1000,
		MaxTtlIdle:  cfg.IdleTimeoutSec * 1000,
	}
}
