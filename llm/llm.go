package llm

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"text/template"
	"time"
)

// --- Embedding Templates and Snippets ---

//go:embed prompts/base_system_prompt.md
var basePromptTemplateString string

//go:embed prompts/snippets
var snippetRootFS embed.FS

// Global template object, parsed once for efficiency
var systemPromptTmpl *template.Template

func init() {
	var err error
	systemPromptTmpl, err = template.New("systemPrompt").Parse(basePromptTemplateString)
	if err != nil {
		panic(fmt.Errorf("failed to parse base system prompt template: %w", err))
	}
}

// PromptTemplateData is the data structrue passed to the template
type PromptTemplateData struct {
	MutationTypeName      string
	FewShotExamples       string
	MutationTypeExplainer string
	CommentMarker         string
	TaskDefinition        string
}

// APIRequest represents the structure of the request payload for the LLM API.
type APIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	TopP        float64   `json:"top_p"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// APIResponse represents the structure of the expected response from the LLM API.
type APIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	// TODO: What about other fields like Usage, Error, etc.
}

// AnalyzeMutation constructs and sends a request to the local LLM API.
func AnalyzeMutation(ctx MutationAnalysisContext) (string, error) {
	// TODO: This was moved above the construction of system prompt purely to get test data
	userContent := fmt.Sprintf(
		"Mutation Type: %s\n\n**Input Code Diff**:\n```diff\n%s\n```\n\n**Input Function Context**:\n```solidity\n%s\n```\n\n",
		ctx.MutationType,
		ctx.MutationDiff,
		ctx.MutationContext,
	)

	fmt.Printf("\033[32m%s\033[0m", userContent)

	customizedSystemPrompt, err := getCustomSystemPrompt(ctx.MutationType)
	if err != nil {
		return "", fmt.Errorf("Failure loading embedded system prompts: %s", err)
	}

	// TODO: Before constructing the request, send GET v1/models to query
	// available models.

	// 4. Construct the API request payload
	apiRequest := APIRequest{
		// Model: "deepseek-r1-distill-qwen-7b",
		Model: "qwen2.5-coder-7b-instruct",
		Messages: []Message{
			{Role: "system", Content: customizedSystemPrompt},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.3,
		MaxTokens:   -1, // No limit
	}

	requestBody, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 5. Make the HTTP POST request
	defer TrackTime(time.Now(), "Calling an LLM")
	llmEndpoint := "http://127.0.0.1:1234/v1/chat/completions"
	resp, err := http.Post(llmEndpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to make POST request to %s: %w", llmEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %s: %s", resp.Status, string(responseBodyBytes))
	}

	// 6. Decode the response
	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", fmt.Errorf("failed to decode API response: %w", err)
	}

	// 7. Extract and return the assistant's message
	if len(apiResponse.Choices) > 0 && apiResponse.Choices[0].Message.Role == "assistant" {
		fmt.Println("\n--- LLM Analysis Result ---")
		fmt.Printf("\033[33m" + apiResponse.Choices[0].Message.Content + "\033[0m" + "\n")
		fmt.Println("---------------------------")

		return apiResponse.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no valid assistant message found in response. Full response: %+v", apiResponse)
}

// Helper function to read a snippet file
func readSnippet(snippetDir fs.FS, mutationKey, snippetName string) (string, error) {
	// This part forms the path segment relative to the "prompts/snippets" directory
	// e.g., "BinaryOpMutation/comment_marker.md"
	relativePathWithinSnippetsSubDir := path.Join(mutationKey, snippetName)

	// Prepend the "prompts/snippets/" prefix because snippetRootFS contains paths starting with it
	fullPathInFS := path.Join("prompts/snippets", relativePathWithinSnippetsSubDir)
	content, err := fs.ReadFile(snippetDir, fullPathInFS)
	if err != nil {
		return "", err
	}
	return string(content), err
}

func getCustomSystemPrompt(mutationTypeKey string) (string, error) {
	data := PromptTemplateData{
		MutationTypeName: mutationTypeKey,
	}

	commentMarker, err := readSnippet(snippetRootFS, mutationTypeKey, "comment_marker.md")
	if err != nil {
		return "", fmt.Errorf("Could not read the comment_marker.md snippet for %s: %s.", mutationTypeKey, err)
	}
	data.CommentMarker = commentMarker

	mutationTypeExplainer, err := readSnippet(snippetRootFS, mutationTypeKey, "mutation_type_explainer.md")
	if err != nil {
		return "", fmt.Errorf("Could not read the mutation_type_explainer.md snippet for %s: %s.", mutationTypeKey, err)
	}
	data.MutationTypeExplainer = mutationTypeExplainer

	taskDefinition, err := readSnippet(snippetRootFS, mutationTypeKey, "task_definition.md")
	if err != nil {
		return "", fmt.Errorf("Could not read the task_definition.md snippet for %s: %s.", mutationTypeKey, err)
	}
	data.TaskDefinition = taskDefinition

	examples, err := readSnippet(snippetRootFS, mutationTypeKey, "examples.md")
	if err != nil {
		return "", fmt.Errorf("Could not read the examples.md snippet for %s: %s.", mutationTypeKey, err)
	}
	data.FewShotExamples = examples

	var populatedPrompt bytes.Buffer

	if err := systemPromptTmpl.Execute(&populatedPrompt, data); err != nil {
		return "", fmt.Errorf("Failed to execute system prompt template for %s: %w", mutationTypeKey, err)
	}

	return populatedPrompt.String(), nil
}

// TrackTime can be used to print the elapsed time it took for a function call
// to perform some logic. Use it with defer keyword before a function call that
// you want to measure. E.g. `defer TrackTime(time.Now(), "Calling an LLM")`
// will print "[Info] Calling an LLM took 25.46 seconds."
func TrackTime(now time.Time, description string) {
	elapsed := time.Since(now).Seconds()
	fmt.Printf("[Info] %s took %.2f seconds.\n", description, elapsed)
}
