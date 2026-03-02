package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LLMProvider is the interface that all LLM providers must implement
type LLMProvider interface {
	// Analyze sends the mutation analysis context to the LLM and returns the response
	Analyze(ctx MutationAnalysisContext, systemPrompt string) (string, error)
	// GetModelName returns the model name being used by this provider
	GetModelName() string
}

// LocalProvider implements LLMProvider for local OpenAI-compatible APIs (Ollama, LM Studio, etc.)
type LocalProvider struct {
	endpoint string
	model    string
}

// NewLocalProvider creates a new LocalProvider instance
func NewLocalProvider(endpoint, model string) *LocalProvider {
	return &LocalProvider{
		endpoint: endpoint,
		model:    model,
	}
}

// GetModelName returns the model name
func (p *LocalProvider) GetModelName() string {
	return p.model
}

// Analyze sends a request to the local LLM API
func (p *LocalProvider) Analyze(ctx MutationAnalysisContext, systemPrompt string) (string, error) {
	userContent := fmt.Sprintf(
		"Mutation Type: %s\n\n**Input Code Diff**:\n```diff\n%s\n```\n\n**Input Function Context**:\n```solidity\n%s\n```\n\n",
		ctx.MutationType,
		ctx.MutationDiff,
		ctx.MutationContext,
	)

	// Print user content for debugging (as before)
	fmt.Printf("\033[32m%s\033[0m", userContent)

	apiRequest := APIRequest{
		Model: p.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.1,
		MaxTokens:   -1, // No limit
	}

	requestBody, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := http.Post(p.endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		// Provide more helpful error message for connection issues
		if strings.Contains(err.Error(), "connection refused") {
			return "", fmt.Errorf("failed to connect to local LLM at %s: connection refused. Make sure your local LLM server (e.g., Ollama, LM Studio) is running and listening on this endpoint", p.endpoint)
		}
		return "", fmt.Errorf("failed to make POST request to %s: %w", p.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %s: %s", resp.Status, string(responseBodyBytes))
	}

	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResponse.Choices) > 0 && apiResponse.Choices[0].Message.Role == "assistant" {
		fmt.Println("\n--- LLM Analysis Result ---")
		fmt.Printf("\033[33m" + apiResponse.Choices[0].Message.Content + "\033[0m" + "\n")
		fmt.Println("---------------------------")
		return apiResponse.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no valid assistant message found in response")
}

// GeminiProvider implements LLMProvider for Google Gemini API
type GeminiProvider struct {
	apiKey string
	model  string
}

// NewGeminiProvider creates a new GeminiProvider instance
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
	}
}

// GetModelName returns the model name
func (p *GeminiProvider) GetModelName() string {
	return p.model
}

// GeminiRequest represents the request structure for Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents a content item in Gemini API
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

// GeminiPart represents a part of content in Gemini API
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents the response structure from Gemini API
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	Error      *GeminiError      `json:"error,omitempty"`
}

// GeminiCandidate represents a candidate response from Gemini API
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// GeminiError represents an error from Gemini API
type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Analyze sends a request to the Google Gemini API
func (p *GeminiProvider) Analyze(ctx MutationAnalysisContext, systemPrompt string) (string, error) {
	// Combine system prompt and user content for Gemini
	// Gemini doesn't have separate system/user messages, so we combine them
	userContent := fmt.Sprintf(
		"Mutation Type: %s\n\n**Input Code Diff**:\n```diff\n%s\n```\n\n**Input Function Context**:\n```solidity\n%s\n```\n\n",
		ctx.MutationType,
		ctx.MutationDiff,
		ctx.MutationContext,
	)

	fullPrompt := systemPrompt + "\n\n" + userContent

	// Gemini API endpoint format: https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", p.model)
	
	// Add API key as query parameter
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Gemini API URL: %w", err)
	}
	q := parsedURL.Query()
	q.Set("key", p.apiKey)
	parsedURL.RawQuery = q.Encode()

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Gemini request body: %w", err)
	}

	resp, err := http.Post(parsedURL.String(), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to make POST request to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Gemini API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var geminiErr GeminiError
		if json.Unmarshal(responseBody, &geminiErr) == nil {
			return "", fmt.Errorf("Gemini API error: %s (status: %s, code: %d)", geminiErr.Message, geminiErr.Status, geminiErr.Code)
		}
		return "", fmt.Errorf("Gemini API request failed with status %s: %s", resp.Status, string(responseBody))
	}

	var geminiResponse GeminiResponse
	if err := json.Unmarshal(responseBody, &geminiResponse); err != nil {
		return "", fmt.Errorf("failed to decode Gemini API response: %w", err)
	}

	if geminiResponse.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s (status: %s, code: %d)", geminiResponse.Error.Message, geminiResponse.Error.Status, geminiResponse.Error.Code)
	}

	if len(geminiResponse.Candidates) > 0 && len(geminiResponse.Candidates[0].Content.Parts) > 0 {
		return geminiResponse.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("no valid response found in Gemini API response")
}

// NewProvider creates a new LLMProvider based on the configuration
func NewProvider(config LLMConfig) (LLMProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid LLM configuration: %w", err)
	}

	switch config.Provider {
	case ProviderTypeLocal:
		return NewLocalProvider(config.Endpoint, config.Model), nil
	case ProviderTypeGemini:
		return NewGeminiProvider(config.APIKey, config.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Provider)
	}
}

