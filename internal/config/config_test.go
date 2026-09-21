package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// configEnvKeys lists every environment variable that maps to a Config field
// (section.field → SECTION_FIELD per viper's key replacer).
var configEnvKeys = []string{
	"APP_NAME",
	"HTTP_ADDR",
	"GRPC_ADDR", "GRPC_READ_TIMEOUT_SEC", "GRPC_WRITE_TIMEOUT_SEC", "GRPC_ENABLE_REFLECTION",
	"LOG_LEVEL",
	"DATADOG_AGENT_ADDR",
	"POSTGRES_MASTER_DSN", "POSTGRES_MASTER_MAX_OPEN", "POSTGRES_MASTER_MAX_IDLE",
	"POSTGRES_MASTER_IDLE_TIMEOUT_SEC", "POSTGRES_MASTER_MAX_LIFETIME_SEC",
	"POSTGRES_SLAVE_DSN", "POSTGRES_SLAVE_MAX_OPEN", "POSTGRES_SLAVE_MAX_IDLE",
	"POSTGRES_SLAVE_IDLE_TIMEOUT_SEC", "POSTGRES_SLAVE_MAX_LIFETIME_SEC",
	"REDIS_URL", "REDIS_POOL_SIZE", "REDIS_MIN_IDLE_CONNS", "REDIS_MAX_IDLE_CONNS",
	"REDIS_CONN_MAX_IDLE_TIME_SEC", "REDIS_CONN_MAX_LIFETIME_SEC", "REDIS_POOL_TIMEOUT_SEC",
	"REDIS_DIAL_TIMEOUT_SEC", "REDIS_READ_TIMEOUT_SEC", "REDIS_WRITE_TIMEOUT_SEC",
	"MONGO_URI", "MONGO_DATABASE", "MONGO_MAX_POOL_SIZE", "MONGO_MIN_POOL_SIZE",
	"MONGO_MAX_CONN_IDLE_TIME_SEC", "MONGO_CONNECT_TIMEOUT_SEC", "MONGO_SERVER_SELECTION_TIMEOUT_SEC",
	"GRPC_SERVICE_BOILERPLATE_ADDR", "GRPC_SERVICE_BOILERPLATE_TLS", "GRPC_SERVICE_BOILERPLATE_TIMEOUT_SEC",
	"GCP_PROJECT_ID",
	"PUBSUB_VENDOR_EVENTS_SUBSCRIPTION_ID",
}

// clearConfigEnv unsets every Config-related env var so a stale value from
// the host shell does not pollute YAML-only assertions. Tests that want to
// verify env-override behaviour must NOT call this; they should call
// t.Setenv after Load is set up via loadFromExample.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range configEnvKeys {
		if _, ok := os.LookupEnv(k); ok {
			t.Setenv(k, "") // t.Setenv restores on cleanup
		}
	}
}

// loadFromExample copies config.yml.example from the repo root into the
// test working directory as config.yaml so viper's "." search path finds
// it. Chdirs into a tempdir to keep the test hermetic and restores cwd on
// cleanup. Does NOT clear env — caller decides whether to call
// clearConfigEnv first (for YAML-only tests) or set specific overrides
// before this (for override tests).
func loadFromExample(t *testing.T) *Config {
	t.Helper()

	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	repoRoot := filepath.Join(origWD, "..", "..")
	exampleYAML, err := os.ReadFile(filepath.Join(repoRoot, "config.yml.example"))
	require.NoError(t, err, "config.yml.example must exist at repo root")

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.yaml"), exampleYAML, 0o644))
	require.NoError(t, os.Chdir(tmpDir))

	return Load()
}

func TestLoad_ParsesExampleYAML(t *testing.T) {
	clearConfigEnv(t)

	// The committed YAML deliberately leaves credential-bearing connection
	// strings empty — infra injects them via env vars at runtime. This test
	// mirrors that contract by setting the DSN/URL/URI env vars to dev
	// fixtures before Load(), then asserting that (a) the YAML supplied the
	// non-sensitive defaults and (b) the env-injected values flowed through.
	const devPostgresDSN = "postgres://test:test@127.0.0.1:5432/test?sslmode=disable"
	const devRedisURL = "redis://127.0.0.1:6379/0"
	const devMongoURI = "mongodb://test:test@127.0.0.1:27017"
	t.Setenv("POSTGRES_MASTER_DSN", devPostgresDSN)
	t.Setenv("POSTGRES_SLAVE_DSN", devPostgresDSN)
	t.Setenv("REDIS_URL", devRedisURL)
	t.Setenv("MONGO_URI", devMongoURI)

	cfg := loadFromExample(t)

	require.Equal(t, "astro-boilerplate", cfg.App.Name)
	require.Equal(t, ":8080", cfg.Http.Addr)
	require.Equal(t, ":5555", cfg.Grpc.Addr)
	require.Equal(t, 30, cfg.Grpc.ReadTimeoutSec)
	require.False(t, cfg.Grpc.EnableReflection)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "127.0.0.1:8126", cfg.Datadog.AgentAddr)

	require.Equal(t, devPostgresDSN, cfg.Postgres.Master.DSN, "DSN comes from env, not YAML")
	require.Equal(t, devPostgresDSN, cfg.Postgres.Slave.DSN, "DSN comes from env, not YAML")
	require.Equal(t, 25, cfg.Postgres.Master.MaxOpen)
	require.Equal(t, 10, cfg.Postgres.Master.MaxIdle)
	require.Equal(t, 300, cfg.Postgres.Master.IdleTimeoutSec)
	require.Equal(t, 1800, cfg.Postgres.Master.MaxLifetimeSec)

	require.Equal(t, devRedisURL, cfg.Redis.URL, "Redis URL comes from env, not YAML")
	require.Equal(t, 100, cfg.Redis.PoolSize)
	require.Equal(t, 10, cfg.Redis.MinIdleConns)
	require.Equal(t, 30, cfg.Redis.MaxIdleConns)
	require.Equal(t, 1800, cfg.Redis.ConnMaxIdleTimeSec)
	require.Equal(t, 4, cfg.Redis.PoolTimeoutSec)
	require.Equal(t, 5, cfg.Redis.DialTimeoutSec)
	require.Equal(t, 3, cfg.Redis.ReadTimeoutSec)
	require.Equal(t, 3, cfg.Redis.WriteTimeoutSec)

	require.Equal(t, devMongoURI, cfg.Mongo.URI, "Mongo URI comes from env, not YAML")
	require.Equal(t, "astro_boilerplate", cfg.Mongo.Database)
	require.Equal(t, uint64(100), cfg.Mongo.MaxPoolSize)
	require.Equal(t, uint64(5), cfg.Mongo.MinPoolSize)
	require.Equal(t, 300, cfg.Mongo.MaxConnIdleTimeSec)
	require.Equal(t, 10, cfg.Mongo.ConnectTimeoutSec)
	require.Equal(t, 10, cfg.Mongo.ServerSelectionTimeoutSec)

	// The example upstream entry is optional; the committed YAML leaves
	// addr empty so the example external client is skipped at startup.
	require.Empty(t, cfg.GrpcService.Boilerplate.Addr,
		"committed YAML must leave grpc_service.boilerplate.addr empty; "+
			"concrete services point it at their real upstream")
	require.True(t, cfg.GrpcService.Boilerplate.TLS)
	require.Equal(t, 10, cfg.GrpcService.Boilerplate.TimeoutSec)

	require.Equal(t, "dogwood-wharf-316804", cfg.Gcp.ProjectID)
	require.Empty(t, cfg.PubSub.VendorEventsSubscriptionID, "subscriber is optional and empty by default")
}

// TestExampleYAML_DoesNotCommitCredentials asserts the operational contract:
// credential-bearing connection strings (postgres.master.dsn, postgres.slave.dsn,
// redis.url, mongo.uri) MUST be empty in the committed YAML. The values flow
// in from env vars at runtime, set by the infra team. Anyone re-introducing a
// non-empty default into the example yaml will be caught here.
func TestExampleYAML_DoesNotCommitCredentials(t *testing.T) {
	origWD, err := os.Getwd()
	require.NoError(t, err)
	exampleYAML, err := os.ReadFile(filepath.Join(origWD, "..", "..", "config.yml.example"))
	require.NoError(t, err)

	for _, line := range []string{
		`dsn: ""`,
		`url: ""`,
		`uri: ""`,
	} {
		require.Contains(t, string(exampleYAML), line,
			"config.yml.example must leave credential-bearing fields empty (%q); "+
				"infra injects DSN/URL/URI via env vars at runtime", line)
	}
}

func TestLoad_EnvOverrideTakesPrecedenceOverYAML(t *testing.T) {
	// Clear host-shell pollution first, then layer our overrides on top.
	clearConfigEnv(t)
	// Mandatory credential-bearing fields — without these, validateMandatory
	// fatals before we ever assert anything.
	t.Setenv("POSTGRES_MASTER_DSN", "postgres://test:test@127.0.0.1:5432/test?sslmode=disable")
	t.Setenv("POSTGRES_SLAVE_DSN", "postgres://test:test@127.0.0.1:5432/test?sslmode=disable")
	// The actual overrides we want to verify.
	t.Setenv("GRPC_ADDR", ":9999")
	t.Setenv("POSTGRES_MASTER_MAX_OPEN", "99")
	t.Setenv("PUBSUB_VENDOR_EVENTS_SUBSCRIPTION_ID", "vendor-events-test-sub")

	cfg := loadFromExample(t)

	require.Equal(t, ":9999", cfg.Grpc.Addr, "env override should beat yaml for top-level key")
	require.Equal(t, 99, cfg.Postgres.Master.MaxOpen, "env override should beat yaml for nested int key")
	require.Equal(t, "vendor-events-test-sub", cfg.PubSub.VendorEventsSubscriptionID,
		"env override should beat yaml for optional key")
}
