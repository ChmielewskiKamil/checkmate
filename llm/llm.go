package llm

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

//go:embed system_prompt.md
var systemPrompt string

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
func AnalyzeMutation() (string, error) {
	// 1. System prompt is embeded into the binary.
	// 2. Hardcoded function context, diff, and mutation type
	functionContext := `
    // @notice See Governor.sol replicates the logic to handle modified calldata from hooks
    function _queueOperations(
        uint256 proposalId,
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 descriptionHash
    ) internal virtual override returns (uint48) {
        uint256 delay = _timelock.getMinDelay();

        bytes32 salt = _timelockSalt(descriptionHash);
        _timelockIds[proposalId] = _timelock.hashOperationBatch(targets, values, calldatas, 0, salt);
        _timelock.scheduleBatch(targets, values, calldatas, 0, salt, delay);

        /// BinaryOpMutation('+' |==> '*') of: 'return SafeCast.toUint48(block.timestamp + delay);'
        return SafeCast.toUint48(block.timestamp*delay);
    }
    `

	diff := `--- original\n+++ mutant\n@@ -374,7 +374,8 @@\n         _timelockIds[proposalId] = _timelock.hashOperationBatch(targets, values, calldatas, 0, salt);\n         _timelock.scheduleBatch(targets, values, calldatas, 0, salt, delay);\n \n-        return SafeCast.toUint48(block.timestamp + delay);\n+        /// BinaryOpMutation('+' |==> '*') of: 'return SafeCast.toUint48(block.timestamp + delay);'\n+        return SafeCast.toUint48(block.timestamp*delay);\n     }\n \n     // @notice See Governor.sol replicates the logic to handle modified calldata from hooks\n"`

	mutationType := "BinaryOpMutation"

	// 3. Construct the user prompt content
	userContent := fmt.Sprintf(
		"Mutation Type: %s\n\nCode Diff:\n```diff%s\n```\n\nFunction Context:\n```solidity%s\n```",
		mutationType,
		diff,
		functionContext,
	)

	fmt.Println(userContent)

	// 4. Construct the API request payload
	apiRequest := APIRequest{
		Model: "deepseek-r1-distill-qwen-7b",
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.0,
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
		return apiResponse.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no valid assistant message found in response. Full response: %+v", apiResponse)
}

// TrackTime can be used to print the elapsed time it took for a function call
// to perform some logic. Use it with defer keyword before a function call that
// you want to measure. E.g. `defer TrackTime(time.Now(), "Calling an LLM")`
// will print "[Info] Calling an LLM took 25.46 seconds."
func TrackTime(now time.Time, description string) {
	elapsed := time.Since(now).Seconds()
	fmt.Printf("[Info] %s took %.2f seconds.", description, elapsed)
}
