package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emptyLookup(string) (string, bool) { return "", false }

func lookupFrom(env map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestLoad_DefaultsWithNoEnvironment(t *testing.T) {
	cfg, err := load(emptyLookup)
	require.NoError(t, err)

	assert.Equal(t, "", cfg.AnthropicAPIKey)
	assert.Equal(t, "", cfg.OpenRouterAPIKey)
	assert.Equal(t, defaultOpenRouterExtractionModel, cfg.OpenRouterExtractionModel)
	assert.Equal(t, defaultFleetDBPath, cfg.FleetDBPath)
	assert.Equal(t, float64(defaultFleetSpeedMultiplier), cfg.FleetSpeedMultiplier)
	assert.Equal(t, defaultAgentEnabled, cfg.AgentEnabled)
	assert.Equal(t, defaultAgentMaxSteps, cfg.AgentMaxSteps)
	assert.Equal(t, defaultAgentMaxConcurrentTriage, cfg.AgentMaxConcurrentTriage)
	assert.Equal(t, defaultExtractionConfidenceThreshold, cfg.ExtractionConfidenceThreshold)
	assert.Equal(t, defaultPendingActionTTLSeconds, cfg.PendingActionTTLSeconds)
	assert.Equal(t, float64(defaultAgingRate), cfg.AgingRate)
	assert.Equal(t, defaultOperatorID, cfg.OperatorID)
	assert.Equal(t, map[int]float64{1: 1000, 2: 100, 3: 10}, cfg.SeverityWeights)
}

func TestLoad_OverridesFromEnvironment(t *testing.T) {
	cfg, err := load(lookupFrom(map[string]string{
		"AGENT_ENABLED":                   "false",
		"AGENT_MAX_STEPS":                 "12",
		"AGENT_MAX_CONCURRENT_TRIAGE":     "5",
		"EXTRACTION_CONFIDENCE_THRESHOLD": "0.5",
		"PENDING_ACTION_TTL_SECONDS":      "300",
		"AGING_RATE":                      "2.5",
		"FLEET_SPEED_MULTIPLIER":          "60",
		"OPERATOR_ID":                     "operator-2",
		"SEVERITY_WEIGHTS":                `{"1":500,"2":50,"3":5}`,
		"OPENROUTER_EXTRACTION_MODEL":     "some/other-model",
		"FLEET_DB_PATH":                   "/tmp/fleet.db",
	}))
	require.NoError(t, err)

	assert.False(t, cfg.AgentEnabled)
	assert.Equal(t, 12, cfg.AgentMaxSteps)
	assert.Equal(t, 5, cfg.AgentMaxConcurrentTriage)
	assert.Equal(t, 0.5, cfg.ExtractionConfidenceThreshold)
	assert.Equal(t, 300, cfg.PendingActionTTLSeconds)
	assert.Equal(t, 2.5, cfg.AgingRate)
	assert.Equal(t, 60.0, cfg.FleetSpeedMultiplier)
	assert.Equal(t, "operator-2", cfg.OperatorID)
	assert.Equal(t, map[int]float64{1: 500, 2: 50, 3: 5}, cfg.SeverityWeights)
	assert.Equal(t, "some/other-model", cfg.OpenRouterExtractionModel)
	assert.Equal(t, "/tmp/fleet.db", cfg.FleetDBPath)
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	cases := map[string]map[string]string{
		"threshold above 1":       {"EXTRACTION_CONFIDENCE_THRESHOLD": "1.5"},
		"threshold below 0":       {"EXTRACTION_CONFIDENCE_THRESHOLD": "-0.1"},
		"non-positive max steps":  {"AGENT_MAX_STEPS": "0"},
		"non-positive triage cap": {"AGENT_MAX_CONCURRENT_TRIAGE": "-1"},
		"non-positive TTL":        {"PENDING_ACTION_TTL_SECONDS": "0"},
		"non-positive speed":      {"FLEET_SPEED_MULTIPLIER": "0"},
		"negative aging rate":     {"AGING_RATE": "-1"},
		"malformed severity JSON": {"SEVERITY_WEIGHTS": "not-json"},
		"missing severity key":    {"SEVERITY_WEIGHTS": `{"1":100,"2":10}`},
		"non-numeric int":         {"AGENT_MAX_STEPS": "abc"},
		"non-numeric float":       {"AGING_RATE": "abc"},
		"non-bool":                {"AGENT_ENABLED": "yesplease"},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(lookupFrom(env))
			require.Error(t, err)
		})
	}
}

func TestEnvLookup_FallsBackToDotEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte(""+
		"# a comment\n"+
		"\n"+
		"OPENROUTER_API_KEY=from-dotenv\n"+
		"OPENROUTER_EXTRACTION_MODEL= # default: some/model\n"+
		"OPERATOR_ID=\"quoted-value\"\n",
	), 0o600))

	t.Setenv("OPENROUTER_API_KEY", "from-process-env")

	lookup := envLookup(path)

	v, ok := lookup("OPENROUTER_API_KEY")
	assert.True(t, ok)
	assert.Equal(t, "from-process-env", v, "process env must win over .env")

	v, ok = lookup("OPERATOR_ID")
	assert.True(t, ok)
	assert.Equal(t, "quoted-value", v, "surrounding quotes are stripped")

	v, ok = lookup("OPENROUTER_EXTRACTION_MODEL")
	assert.True(t, ok)
	assert.Equal(t, "", v, "a bare '# comment' value parses as empty, not the comment text")

	_, ok = lookup("FLEET_DB_PATH")
	assert.False(t, ok, "a key absent from both process env and .env stays unset")
}

func TestEnvLookup_MissingDotEnvFileIsNotAnError(t *testing.T) {
	lookup := envLookup(filepath.Join(t.TempDir(), "does-not-exist.env"))

	_, ok := lookup("OPENROUTER_API_KEY")
	assert.False(t, ok)
}
