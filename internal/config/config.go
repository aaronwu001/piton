// Package config reads the one YAML configuration file the orchestrator boots
// on (SPEC.md 4.4).
//
// SPEC.md 4.4 fixes WHAT the file must declare — "the storage backend and its
// connection details, the HTTP listen address, the sweep interval, the
// heartbeat interval and the lease TTL" — and does not fix the key names. The
// names below are demos/alpha/piton.yaml's, which is the file this milestone
// boots on; that file's own header records the two conventions they follow.
//
// SPEC.md 9.1: configuration files on disk are YAML.
package config

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Storage names the backend and how to reach it. SPEC.md 4.4 requires the
// backend to be a value in this file "in support of the storage abstraction of
// SPEC.md 7 — Postgres is today's only implementation, not a hard-coded
// assumption".
type Storage struct {
	Backend string `yaml:"backend"`
	DSN     string `yaml:"dsn"`
}

// HTTP is the listen address of the API surface of SPEC.md 10.
type HTTP struct {
	ListenAddress string `yaml:"listen_address"`
}

// Config is the whole file.
type Config struct {
	Storage Storage `yaml:"storage"`
	HTTP    HTTP    `yaml:"http"`

	// SweepIntervalSeconds is SPEC.md 8.6's sweep interval, 5 seconds by
	// default. It is also the width of SPEC.md 13.3's uncertainty window.
	SweepIntervalSeconds int `yaml:"sweep_interval_seconds"`

	// HeartbeatIntervalSeconds and LeaseTTLSeconds are SPEC.md 8.7's pair: an
	// orchestrator writes one heartbeat row every interval, and is live iff
	// last_seen_at > now() - lease_ttl. Defaults 10 s and 30 s.
	HeartbeatIntervalSeconds int `yaml:"heartbeat_interval_seconds"`
	LeaseTTLSeconds          int `yaml:"lease_ttl_seconds"`
}

// The defaults are SPEC.md 8.6 and 8.7's stated values. They apply only to a
// key the file omits; a key present with an out-of-range value is an error and
// is never silently replaced — SPEC.md 11.2's "a rejection is a 400, never
// silence" is about the API, but its reason ("a setting silently ignored makes
// the user believe it took effect") applies just as directly to a
// configuration file the operator wrote by hand.
const (
	DefaultSweepIntervalSeconds     = 5
	DefaultHeartbeatIntervalSeconds = 10
	DefaultLeaseTTLSeconds          = 30
)

// Load reads and validates one configuration file.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	// An unknown key is a typo, and a typo silently ignored is the failure
	// mode SPEC.md 16 rule 4 exists to prevent.
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("config: cannot parse %s: %w", path, err)
	}

	if c.SweepIntervalSeconds == 0 {
		c.SweepIntervalSeconds = DefaultSweepIntervalSeconds
	}
	if c.HeartbeatIntervalSeconds == 0 {
		c.HeartbeatIntervalSeconds = DefaultHeartbeatIntervalSeconds
	}
	if c.LeaseTTLSeconds == 0 {
		c.LeaseTTLSeconds = DefaultLeaseTTLSeconds
	}

	switch {
	case c.Storage.Backend == "":
		return nil, fmt.Errorf("config: %s declares no storage.backend (SPEC.md 4.4)", path)
	case c.Storage.DSN == "":
		return nil, fmt.Errorf("config: %s declares no storage.dsn (SPEC.md 4.4)", path)
	case c.HTTP.ListenAddress == "":
		return nil, fmt.Errorf("config: %s declares no http.listen_address (SPEC.md 4.4)", path)
	case c.SweepIntervalSeconds < 1:
		return nil, fmt.Errorf("config: sweep_interval_seconds must be >= 1, got %d", c.SweepIntervalSeconds)
	case c.HeartbeatIntervalSeconds < 1:
		return nil, fmt.Errorf("config: heartbeat_interval_seconds must be >= 1, got %d", c.HeartbeatIntervalSeconds)
	case c.LeaseTTLSeconds < 1:
		return nil, fmt.Errorf("config: lease_ttl_seconds must be >= 1, got %d", c.LeaseTTLSeconds)
	}

	// SPEC.md 8.7 derives liveness from the two together, and says why 30 s
	// pairs with 10 s: it "tolerates two missed heartbeats before another
	// orchestrator may take over". A TTL at or below the interval tolerates
	// none, so an orchestrator would be declared dead by its own scheduling
	// jitter and its runs stolen while it was driving them.
	if c.LeaseTTLSeconds <= c.HeartbeatIntervalSeconds {
		return nil, fmt.Errorf(
			"config: lease_ttl_seconds (%d) must exceed heartbeat_interval_seconds (%d) (SPEC.md 8.7)",
			c.LeaseTTLSeconds, c.HeartbeatIntervalSeconds)
	}
	return &c, nil
}

// SweepInterval, HeartbeatInterval and LeaseTTL hand the engine durations, so
// that no caller repeats the seconds-to-Duration conversion.
func (c *Config) SweepInterval() time.Duration {
	return time.Duration(c.SweepIntervalSeconds) * time.Second
}

func (c *Config) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalSeconds) * time.Second
}

func (c *Config) LeaseTTL() time.Duration {
	return time.Duration(c.LeaseTTLSeconds) * time.Second
}
