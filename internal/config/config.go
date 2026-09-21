package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Load reads config.yaml from the working directory and returns a populated
// *Config. Any individual key can be overridden by an environment variable
// using the convention `section.field` → `SECTION_FIELD` (nested keys join
// with underscores). For example, `postgres.master.address` is overridden
// by `POSTGRES_MASTER_ADDRESS`.
//
// Missing or unreadable config aborts the process via log.Fatalf — callers
// should invoke Load() at the top of main and let the process exit on
// misconfig. Developers seed a local config.yaml by copying the committed
// template at the repo root (`cp config.yml.example config.yaml`).
func Load() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("config: read %s: %v", v.ConfigFileUsed(), err)
	}

	// Environment-variable overrides. AutomaticEnv consults env vars during
	// Unmarshal; the key replacer turns `section.field` into `SECTION_FIELD`
	// so the YAML schema and env override surface stay in lockstep.
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		log.Fatalf("config: unmarshal: %v", err)
	}

	validateMandatory(cfg)
	return cfg
}

// validateMandatory enforces that every key the service cannot run without is
// present. Optional sections (Redis, Mongo, PubSub subscription) are not
// validated — their consumers handle empty values via NoOp fallbacks.
//
// A mandatory key missing here means the YAML schema is incomplete or an env
// override accidentally cleared a value. Either way we want to fail fast at
// startup rather than at first use.
func validateMandatory(cfg *Config) {
	required := []struct {
		key string
		val string
	}{
		{"app.name", cfg.App.Name},
		{"http.addr", cfg.Http.Addr},
		{"grpc.addr", cfg.Grpc.Addr},
		{"datadog.agent_addr", cfg.Datadog.AgentAddr},
		{"postgres.master.dsn", cfg.Postgres.Master.DSN},
		{"postgres.slave.dsn", cfg.Postgres.Slave.DSN},
		{"gcp.project_id", cfg.Gcp.ProjectID},
	}
	for _, r := range required {
		if r.val == "" {
			log.Fatalf("config: missing mandatory key %s", r.key)
		}
	}

	requiredInts := []struct {
		key string
		val int
	}{
		{"postgres.master.max_open", cfg.Postgres.Master.MaxOpen},
		{"postgres.master.max_idle", cfg.Postgres.Master.MaxIdle},
		{"postgres.master.idle_timeout_sec", cfg.Postgres.Master.IdleTimeoutSec},
		{"postgres.master.max_lifetime_sec", cfg.Postgres.Master.MaxLifetimeSec},
		{"postgres.slave.max_open", cfg.Postgres.Slave.MaxOpen},
		{"postgres.slave.max_idle", cfg.Postgres.Slave.MaxIdle},
		{"postgres.slave.idle_timeout_sec", cfg.Postgres.Slave.IdleTimeoutSec},
		{"postgres.slave.max_lifetime_sec", cfg.Postgres.Slave.MaxLifetimeSec},
	}
	for _, r := range requiredInts {
		if r.val <= 0 {
			log.Fatalf("config: missing or non-positive mandatory key %s", r.key)
		}
	}
}
