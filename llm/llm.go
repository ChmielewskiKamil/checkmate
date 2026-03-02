package llm

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
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

// AnalyzeMutation analyzes a mutation using the provided LLM provider
func AnalyzeMutation(ctx MutationAnalysisContext, provider LLMProvider) (string, error) {
	customizedSystemPrompt, err := getCustomSystemPrompt(ctx.MutationType)
	if err != nil {
		return "", fmt.Errorf("failure loading embedded system prompts: %w", err)
	}

	// Delegate to the provider for actual API communication
	response, err := provider.Analyze(ctx, customizedSystemPrompt)
	if err != nil {
		return "", fmt.Errorf("LLM analysis failed: %w", err)
	}

	return response, nil
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
