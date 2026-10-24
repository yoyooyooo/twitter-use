package config

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the normalized runtime configuration.
type Config struct {
	Fetch struct {
		Count int `yaml:"count"`
	} `yaml:"fetch"`
	Filter struct {
		Mode            string             `yaml:"mode"`
		TopN            int                `yaml:"topN"`
		MinScore        float64            `yaml:"minScore"`
		Lang            []string           `yaml:"lang"`
		ExcludeRetweets bool               `yaml:"excludeRetweets"`
		Weights         map[string]float64 `yaml:"weights"`
	} `yaml:"filter"`
	RateLimit struct {
		RequestDelay   float64 `yaml:"requestDelay"`
		MaxRetries     int     `yaml:"maxRetries"`
		RetryBaseDelay float64 `yaml:"retryBaseDelay"`
	} `yaml:"rateLimit"`
}

// Default returns the built-in configuration. maxCount is intentionally absent.
func Default() Config {
	cfg := Config{}
	cfg.Fetch.Count = 50
	cfg.Filter.Mode = "topN"
	cfg.Filter.TopN = 20
	cfg.Filter.MinScore = 50
	cfg.Filter.Weights = map[string]float64{
		"likes": 1, "retweets": 3, "replies": 2, "bookmarks": 5, "views_log": 0.5,
	}
	cfg.RateLimit.RequestDelay = 2.5
	cfg.RateLimit.MaxRetries = 3
	cfg.RateLimit.RetryBaseDelay = 5
	return cfg
}

// Load reads YAML from an explicit path, ./config.yaml, or the defaults.
func Load(path string) Config {
	cfg := Default()
	resolved := path
	if resolved == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			resolved = "config.yaml"
		}
	}
	if resolved == "" {
		return cfg
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		log.Printf("Failed to read config file %s: %s", resolved, err)
		return cfg
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		log.Printf("Failed to parse YAML config %s: %s", resolved, err)
		return cfg
	}
	if parsed == nil {
		return cfg
	}
	delete(asMap(parsed["rateLimit"]), "maxCount")
	normalized, err := yaml.Marshal(parsed)
	if err != nil {
		return cfg
	}
	if err := yaml.Unmarshal(normalized, &cfg); err != nil {
		log.Printf("Failed to apply config %s: %s", resolved, err)
		return Default()
	}
	return normalize(cfg)
}

func normalize(cfg Config) Config {
	base := Default()
	if cfg.Fetch.Count < 1 {
		cfg.Fetch.Count = base.Fetch.Count
	}
	switch cfg.Filter.Mode {
	case "topN", "score", "all":
	default:
		cfg.Filter.Mode = "topN"
	}
	if cfg.Filter.TopN < 1 {
		cfg.Filter.TopN = base.Filter.TopN
	}
	if cfg.Filter.MinScore == 0 {
		cfg.Filter.MinScore = base.Filter.MinScore
	}
	if cfg.Filter.Weights == nil {
		cfg.Filter.Weights = map[string]float64{}
	}
	for key, value := range base.Filter.Weights {
		if _, ok := cfg.Filter.Weights[key]; !ok {
			cfg.Filter.Weights[key] = value
		}
	}
	if cfg.RateLimit.RetryBaseDelay < 1 {
		cfg.RateLimit.RetryBaseDelay = base.RateLimit.RetryBaseDelay
	}
	if cfg.RateLimit.RequestDelay < 0 {
		cfg.RateLimit.RequestDelay = 0
	}
	if cfg.RateLimit.MaxRetries < 0 {
		cfg.RateLimit.MaxRetries = 0
	}
	return cfg
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

// PathNextTo is unused by callers; kept so tests can point at a fixture directory.
func PathNextTo(dir, name string) string { return filepath.Join(dir, name) }
