package llm

import (
	"fmt"
	"os"
	"strings"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderTypeLocal  ProviderType = "local"
	ProviderTypeGemini ProviderType = "gemini"
)

// LLMConfig holds configuration for the LLM provider
type LLMConfig struct {
	Provider ProviderType // "local" or "gemini"
	Endpoint string       // API endpoint URL (for local provider)
	Model    string       // Model name/identifier
	APIKey   string       // API key (for Gemini provider)
}

// LoadConfigFromEnv loads LLM configuration from environment variables
func LoadConfigFromEnv() LLMConfig {
	config := LLMConfig{
		Provider: ProviderTypeLocal,
		Endpoint: "http://127.0.0.1:11434/v1/chat/completions",
		Model:    "qwen:latest",
	}

	// Provider type
	if providerStr := os.Getenv("CHECKMATE_LLM_PROVIDER"); providerStr != "" {
		providerStr = strings.ToLower(strings.TrimSpace(providerStr))
		switch providerStr {
		case "local":
			config.Provider = ProviderTypeLocal
		case "gemini":
			config.Provider = ProviderTypeGemini
		default:
			// Invalid provider, keep default
		}
	}

	// Endpoint (for local provider)
	if endpoint := os.Getenv("CHECKMATE_LLM_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
	}

	// Model
	if model := os.Getenv("CHECKMATE_LLM_MODEL"); model != "" {
		config.Model = model
	}

	// API Key (for Gemini)
	if apiKey := os.Getenv("CHECKMATE_LLM_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}

	return config
}

// Validate checks if the configuration is valid
func (c *LLMConfig) Validate() error {
	if c.Provider != ProviderTypeLocal && c.Provider != ProviderTypeGemini {
		return fmt.Errorf("invalid provider type: %s (must be 'local' or 'gemini')", c.Provider)
	}

	if c.Provider == ProviderTypeGemini {
		if c.APIKey == "" {
			return fmt.Errorf("API key is required for Gemini provider")
		}
		if c.Model == "" {
			return fmt.Errorf("model name is required for Gemini provider")
		}
	} else if c.Provider == ProviderTypeLocal {
		if c.Endpoint == "" {
			return fmt.Errorf("endpoint is required for local provider")
		}
		if c.Model == "" {
			return fmt.Errorf("model name is required for local provider")
		}
	}

	return nil
}

// MergeWithFlags merges environment config with CLI flag values (flags take priority)
func (c *LLMConfig) MergeWithFlags(provider, endpoint, model, apiKey string) {
	if provider != "" {
		provider = strings.ToLower(strings.TrimSpace(provider))
		switch provider {
		case "local":
			c.Provider = ProviderTypeLocal
		case "gemini":
			c.Provider = ProviderTypeGemini
		}
	}

	if endpoint != "" {
		c.Endpoint = endpoint
	}

	if model != "" {
		c.Model = model
	}

	if apiKey != "" {
		c.APIKey = apiKey
	}
}

