package config

type (
	// Config aggregates every configuration knob the service understands.
	// Loaded from config.yaml in the working directory (template lives at
	// the repo root as config.yml.example); individual keys can be
	// overridden by environment variables using the convention
	// `section.field` → `SECTION_FIELD` (nested keys join with underscores).
	//
	// Sections marked optional in validateMandatory() may carry zero-valued
	// sub-structs without aborting startup.
	Config struct {
		App         AppConfig           `mapstructure:"app"`
		Http        HttpConfig          `mapstructure:"http"`
		Grpc        GrpcConfig          `mapstructure:"grpc"`
		Log         LoggingConfig       `mapstructure:"log"`
		Datadog     DatadogConfig       `mapstructure:"datadog"`
		Postgres    PostgresConfig      `mapstructure:"postgres"`
		Redis       RedisConfig         `mapstructure:"redis"`
		Mongo       MongoConfig         `mapstructure:"mongo"`
		GrpcService ExternalGrpcService `mapstructure:"grpc_service"`
		Gcp         GcpConfig           `mapstructure:"gcp"`
		PubSub      PubSubConfig        `mapstructure:"pubsub"`
	}

	AppConfig struct {
		Name string `mapstructure:"name"`
	}

	HttpConfig struct {
		Addr string `mapstructure:"addr"`
	}

	GrpcConfig struct {
		Addr             string `mapstructure:"addr"`
		ReadTimeoutSec   int    `mapstructure:"read_timeout_sec"`
		WriteTimeoutSec  int    `mapstructure:"write_timeout_sec"`
		EnableReflection bool   `mapstructure:"enable_reflection"`
	}

	LoggingConfig struct {
		Level string `mapstructure:"level"`
	}

	DatadogConfig struct {
		AgentAddr string `mapstructure:"agent_addr"`
	}

	// PostgresConfig holds the master/slave pair used by the read/write split.
	PostgresConfig struct {
		Master PostgresInstance `mapstructure:"master"`
		Slave  PostgresInstance `mapstructure:"slave"`
	}

	// PostgresInstance holds the full DSN (injected by infra; the service must
	// not construct it) plus pool tunables that the service owns.
	//
	// DSN format follows libpq / pgx: `postgres://user:pass@host:port/db?sslmode=...`.
	// Credentials and host belong to the DSN — never split them into separate
	// fields here, that responsibility lives with whoever provisions the
	// connection string.
	PostgresInstance struct {
		// DSN is injected by infra at runtime. Env var depends on which
		// instance: POSTGRES_MASTER_DSN for Master, POSTGRES_SLAVE_DSN for
		// Slave (viper.AutomaticEnv maps `postgres.master.dsn` →
		// `POSTGRES_MASTER_DSN` via the dot→underscore replacer set in
		// config.go's Load). Committed YAML keeps this empty.
		DSN            string `mapstructure:"dsn"`
		MaxOpen        int    `mapstructure:"max_open"`
		MaxIdle        int    `mapstructure:"max_idle"`
		IdleTimeoutSec int    `mapstructure:"idle_timeout_sec"`
		MaxLifetimeSec int    `mapstructure:"max_lifetime_sec"`
	}

	// RedisConfig is optional; an empty URL signals "Redis disabled" and the
	// composition root falls back to a no-op lock implementation.
	//
	// URL format follows go-redis / Redis URI scheme:
	// `redis://user:pass@host:port/db` (or `rediss://...` for TLS). Credentials,
	// host, port, and database number all live in the URL — the service must
	// not assemble them from parts. Everything else on this struct is pool
	// behaviour that the service tunes per workload.
	RedisConfig struct {
		// URL is injected by infra at runtime via the REDIS_URL env var
		// (viper.AutomaticEnv maps `redis.url` → `REDIS_URL`). Committed
		// YAML keeps this empty; an empty resolved value at startup
		// signals "Redis disabled" and the composition root falls back to
		// the NoOp lock implementation.
		URL                string `mapstructure:"url"`
		PoolSize           int    `mapstructure:"pool_size"`
		MinIdleConns       int    `mapstructure:"min_idle_conns"`
		MaxIdleConns       int    `mapstructure:"max_idle_conns"`
		ConnMaxIdleTimeSec int    `mapstructure:"conn_max_idle_time_sec"`
		ConnMaxLifetimeSec int    `mapstructure:"conn_max_lifetime_sec"`
		PoolTimeoutSec     int    `mapstructure:"pool_timeout_sec"`
		DialTimeoutSec     int    `mapstructure:"dial_timeout_sec"`
		ReadTimeoutSec     int    `mapstructure:"read_timeout_sec"`
		WriteTimeoutSec    int    `mapstructure:"write_timeout_sec"`
	}

	// MongoConfig is optional; an empty URI signals "Mongo disabled".
	//
	// URI format: `mongodb://user:pass@host:port/?authSource=...` (or
	// `mongodb+srv://...`). The URI is the connection identity — credentials,
	// hosts, replica-set name all live there. `Database` is the application's
	// choice of database within the cluster (passed to `client.Database()` at
	// use time), not part of the connection string. Pool tunables are
	// service-owned.
	MongoConfig struct {
		// URI is injected by infra at runtime via the MONGO_URI env var
		// (viper.AutomaticEnv maps `mongo.uri` → `MONGO_URI`). Committed
		// YAML keeps this empty; an empty resolved value at startup
		// signals "Mongo disabled" and the composition root substitutes
		// the NoOp audit-log writer.
		URI                       string `mapstructure:"uri"`
		Database                  string `mapstructure:"database"`
		MaxPoolSize               uint64 `mapstructure:"max_pool_size"`
		MinPoolSize               uint64 `mapstructure:"min_pool_size"`
		MaxConnIdleTimeSec        int    `mapstructure:"max_conn_idle_time_sec"`
		ConnectTimeoutSec         int    `mapstructure:"connect_timeout_sec"`
		ServerSelectionTimeoutSec int    `mapstructure:"server_selection_timeout_sec"`
	}

	ExternalGrpcService struct {
		// Boilerplate is an example upstream gRPC service entry. Empty Addr
		// ⇒ the example external client (see
		// internal/infrastructure/external/boilerplate) is skipped at
		// startup. Concrete services add one entry per real upstream they
		// depend on.
		Boilerplate GrpcServiceConfig `mapstructure:"boilerplate"`
	}

	GrpcServiceConfig struct {
		Addr       string `mapstructure:"addr"`
		TLS        bool   `mapstructure:"tls"`
		TimeoutSec int    `mapstructure:"timeout_sec"`
	}

	GcpConfig struct {
		ProjectID string `mapstructure:"project_id"`
	}

	// PubSubConfig is optional; an empty VendorEventsSubscriptionID signals
	// "subscriber disabled" and the composition root skips registering the
	// inbound vendor-event consumer. The boilerplate boots without GCP
	// credentials when this is unset.
	PubSubConfig struct {
		VendorEventsSubscriptionID string `mapstructure:"vendor_events_subscription_id"`
	}
)
