// Package config parses and validates every env-driven constant listed in
// CLAUDE.md's config table (§ "Config constants") plus the fleet-service
// specific vars in .env.example. Nothing in the rest of the repo reads
// os.Getenv directly — code reads a Config field instead, so tests can
// inject values without touching the environment.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is every environment-driven constant the fleet service needs at
// boot. Zero-value-safe defaults are documented in .env.example and applied
// by Load when a variable is unset or empty.
type Config struct {
	AnthropicAPIKey               string
	OpenRouterAPIKey              string
	OpenRouterExtractionModel     string
	FleetDBPath                   string
	FleetSpeedMultiplier          float64
	AgentEnabled                  bool
	AgentMaxSteps                 int
	AgentMaxConcurrentTriage      int
	ExtractionConfidenceThreshold float64
	PendingActionTTLSeconds       int
	SeverityWeights               map[int]float64
	AgingRate                     float64
	OperatorID                    string
	FleetPort                     int
	AgentBaseURL                  string
}

const (
	defaultOpenRouterExtractionModel     = "google/gemma-4-26b-a4b-it:free" // DEC-013
	defaultFleetDBPath                   = "fleet.db"
	defaultFleetSpeedMultiplier          = 1.0
	defaultAgentEnabled                  = true
	defaultAgentMaxSteps                 = 8            // OQ-11
	defaultAgentMaxConcurrentTriage      = 2            // DEC-018
	defaultExtractionConfidenceThreshold = 0.75         // OQ-4
	defaultPendingActionTTLSeconds       = 600          // DEC-019
	defaultAgingRate                     = 1.0          // OQ-5
	defaultOperatorID                    = "operator-1"            // OQ-7
	defaultFleetPort                     = 8080                    // matches agent's default FLEET_GRAPHQL_URL
	defaultAgentBaseURL                  = "http://localhost:8081" // matches agent's default AGENT_PORT
)

// defaultSeverityWeights is OQ-5's recorded default: severity_weight =
// {1: 1000, 2: 100, 3: 10}.
func defaultSeverityWeights() map[int]float64 {
	return map[int]float64{1: 1000, 2: 100, 3: 10}
}

// Load reads every config var from the process environment, applying the
// recorded defaults (CLAUDE.md's config table, the Open Questions register)
// for anything unset. It never fails on missing API keys — those are only
// required by make test:live, and make test must pass with none set.
//
// Real environment variables always take precedence over a .env file; .env
// is a local dev convenience (never read by make test, which runs with none
// of these vars set and must still pass).
func Load() (Config, error) {
	return load(envLookup(".env"))
}

type lookupFunc func(key string) (string, bool)

// envLookup returns a lookupFunc backed by the process environment, falling
// back to the given .env file (if present) for anything not already set in
// the environment.
func envLookup(dotenvPath string) lookupFunc {
	dotenv := loadDotEnv(dotenvPath)
	return func(key string) (string, bool) {
		if v, ok := os.LookupEnv(key); ok {
			return v, true
		}
		v, ok := dotenv[key]
		return v, ok
	}
}

// loadDotEnv does a best-effort parse of a simple KEY=VALUE .env file.
// A missing file, or any line it can't parse, is silently skipped — this is
// a dev convenience, not a config format that needs to be enforced.
func loadDotEnv(path string) map[string]string {
	values := map[string]string{}

	data, err := os.ReadFile(path)
	if err != nil {
		return values
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if strings.HasPrefix(val, "#") {
			val = ""
		} else if idx := strings.Index(val, " #"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}
		val = strings.Trim(val, `"'`)
		if key != "" {
			values[key] = val
		}
	}
	return values
}

func load(lookup lookupFunc) (Config, error) {
	cfg := Config{
		AnthropicAPIKey:           getString(lookup, "ANTHROPIC_API_KEY", ""),
		OpenRouterAPIKey:          getString(lookup, "OPENROUTER_API_KEY", ""),
		OpenRouterExtractionModel: getString(lookup, "OPENROUTER_EXTRACTION_MODEL", defaultOpenRouterExtractionModel),
		FleetDBPath:               getString(lookup, "FLEET_DB_PATH", defaultFleetDBPath),
		OperatorID:                getString(lookup, "OPERATOR_ID", defaultOperatorID),
		AgentBaseURL:              getString(lookup, "AGENT_BASE_URL", defaultAgentBaseURL),
	}

	var err error
	if cfg.FleetPort, err = getInt(lookup, "FLEET_PORT", defaultFleetPort); err != nil {
		return Config{}, err
	}
	if cfg.FleetSpeedMultiplier, err = getFloat(lookup, "FLEET_SPEED_MULTIPLIER", defaultFleetSpeedMultiplier); err != nil {
		return Config{}, err
	}
	if cfg.AgentEnabled, err = getBool(lookup, "AGENT_ENABLED", defaultAgentEnabled); err != nil {
		return Config{}, err
	}
	if cfg.AgentMaxSteps, err = getInt(lookup, "AGENT_MAX_STEPS", defaultAgentMaxSteps); err != nil {
		return Config{}, err
	}
	if cfg.AgentMaxConcurrentTriage, err = getInt(lookup, "AGENT_MAX_CONCURRENT_TRIAGE", defaultAgentMaxConcurrentTriage); err != nil {
		return Config{}, err
	}
	if cfg.ExtractionConfidenceThreshold, err = getFloat(lookup, "EXTRACTION_CONFIDENCE_THRESHOLD", defaultExtractionConfidenceThreshold); err != nil {
		return Config{}, err
	}
	if cfg.PendingActionTTLSeconds, err = getInt(lookup, "PENDING_ACTION_TTL_SECONDS", defaultPendingActionTTLSeconds); err != nil {
		return Config{}, err
	}
	if cfg.AgingRate, err = getFloat(lookup, "AGING_RATE", defaultAgingRate); err != nil {
		return Config{}, err
	}
	if cfg.SeverityWeights, err = getSeverityWeights(lookup); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.ExtractionConfidenceThreshold < 0 || c.ExtractionConfidenceThreshold > 1 {
		return fmt.Errorf("config: EXTRACTION_CONFIDENCE_THRESHOLD must be in [0,1], got %v", c.ExtractionConfidenceThreshold)
	}
	if c.AgentMaxSteps <= 0 {
		return fmt.Errorf("config: AGENT_MAX_STEPS must be positive, got %d", c.AgentMaxSteps)
	}
	if c.AgentMaxConcurrentTriage <= 0 {
		return fmt.Errorf("config: AGENT_MAX_CONCURRENT_TRIAGE must be positive, got %d", c.AgentMaxConcurrentTriage)
	}
	if c.PendingActionTTLSeconds <= 0 {
		return fmt.Errorf("config: PENDING_ACTION_TTL_SECONDS must be positive, got %d", c.PendingActionTTLSeconds)
	}
	if c.FleetSpeedMultiplier <= 0 {
		return fmt.Errorf("config: FLEET_SPEED_MULTIPLIER must be positive, got %v", c.FleetSpeedMultiplier)
	}
	if c.AgingRate < 0 {
		return fmt.Errorf("config: AGING_RATE must not be negative, got %v", c.AgingRate)
	}
	for severity, weight := range c.SeverityWeights {
		if severity < 1 || severity > 3 {
			return fmt.Errorf("config: SEVERITY_WEIGHTS key %d out of range 1-3", severity)
		}
		if weight < 0 {
			return fmt.Errorf("config: SEVERITY_WEIGHTS[%d] must not be negative, got %v", severity, weight)
		}
	}
	if _, ok := c.SeverityWeights[1]; !ok {
		return fmt.Errorf("config: SEVERITY_WEIGHTS missing required key 1")
	}
	if _, ok := c.SeverityWeights[2]; !ok {
		return fmt.Errorf("config: SEVERITY_WEIGHTS missing required key 2")
	}
	if _, ok := c.SeverityWeights[3]; !ok {
		return fmt.Errorf("config: SEVERITY_WEIGHTS missing required key 3")
	}
	return nil
}

func getString(lookup lookupFunc, key, def string) string {
	if v, ok := lookup(key); ok && v != "" {
		return v
	}
	return def
}

func getFloat(lookup lookupFunc, key string, def float64) (float64, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid float %q: %w", key, v, err)
	}
	return f, nil
}

func getInt(lookup lookupFunc, key string, def int) (int, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return def, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid int %q: %w", key, v, err)
	}
	return i, nil
}

func getBool(lookup lookupFunc, key string, def bool) (bool, error) {
	v, ok := lookup(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s: invalid bool %q: %w", key, v, err)
	}
	return b, nil
}

// getSeverityWeights parses SEVERITY_WEIGHTS as a JSON object mapping
// severity (1-3) to weight, e.g. {"1":1000,"2":100,"3":10} (OQ-5).
func getSeverityWeights(lookup lookupFunc) (map[int]float64, error) {
	v, ok := lookup("SEVERITY_WEIGHTS")
	if !ok || v == "" {
		return defaultSeverityWeights(), nil
	}
	var raw map[string]float64
	if err := json.Unmarshal([]byte(v), &raw); err != nil {
		return nil, fmt.Errorf("config: SEVERITY_WEIGHTS: invalid JSON %q: %w", v, err)
	}
	weights := make(map[int]float64, len(raw))
	for k, w := range raw {
		severity, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("config: SEVERITY_WEIGHTS: invalid severity key %q: %w", k, err)
		}
		weights[severity] = w
	}
	return weights, nil
}
