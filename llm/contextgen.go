package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MutantJSONInfo represents the structure of an object in gambit_results.json
// file.
type MutantJSONInfo struct {
	Description string `json:"description"`
	Diff        string `json:"diff"`
	ID          string `json:"id"`
	Name        string `json:"name"`     // e.g., "mutants/1/src/Vault.sol"
	Original    string `json:"original"` // e.g., "src/Vault.sol"
	// SourceRoot  string `json:"sourceroot"` // Absolute path to original project source
}

type MutationAnalysisContext struct {
	MutationType        string // Mutation operator name e.g. BinaryOpMutation
	MutationDiff        string // Mutation diff for particular mutant. Extracted from gambit_results.json
	MutationContext     string // Snippet of surrounding lines of code
	MutatedFilePathUsed string // Path to mutated file used for context generation
	MutationMarkerLine  int    // Line number with the Mutation comment
}

func AnalyzeMutations(mutantsDirPath string) error {
	/*~*~*~*~*~*~*~*~*~*~*~* Pre-conditions ~*~*~*~*~*~*~*~*~*~*~*/
	// Handle errors for users:
	// - If the mutations directory is nonexistant, return error.
	// - If there are no mutations in the mutations directory, return error.
	// Panic:
	// - Since this is the entrypoint function, there are no strong
	// preconditions that should cause a panic.
	info, err := os.Stat(mutantsDirPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("[Error] The %s path does not exist. Before running the LLM analysis, you might want to run checkmate in standard mode to generate and slay the mutants first.", mutantsDirPath)
	} else if err != nil {
		return fmt.Errorf("[Error] Error accessing path %s: %w", mutantsDirPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("[Error] The %s path exists, but is not a directory.", mutantsDirPath)
	}

	entries, err := os.ReadDir(mutantsDirPath)
	if err != nil {
		return fmt.Errorf("[Error] There was a problem reading directory: %v", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("[Error] The directory is empty: %s", mutantsDirPath)
	}

	// TODO: Check if particular mutant file is not empty (?)

	/*~*~*~*~*~*~*~*~*~*~*~*~* Actions ~*~*~*~*~*~*~*~*~*~*~*~*~*/

	var survivorIds []string
	for _, entry := range entries {
		if entry.IsDir() {
			survivorIds = append(survivorIds, entry.Name())
		}
	}

	// TODO: This can be removed for production build but helps with picking
	// mutations to analyze at random.
	rand.Shuffle(len(survivorIds), func(i, j int) {
		survivorIds[i], survivorIds[j] = survivorIds[j], survivorIds[i]
	})

	// From gambitSurvivorsMap we can extract the Mutation Type and Mutation
	// Diff.
	gambitSurvivorsMap, err := extractGambitResultsJSON(mutantsDirPath)
	if err != nil {
		return fmt.Errorf("[Error] Error extracting mutation results from gambit_results.json: %w", err)
	}

	for _, id := range survivorIds {
		jsonInfo, ok := gambitSurvivorsMap[id]
		if !ok {
			fmt.Printf("[Warning] No JSON entry found for mutant survivor with ID %s. Skipping.\n", id)
			continue
		}
		// 1. Create context
		mutationContext, err := generateMutationAnalysisContext(id, mutantsDirPath, jsonInfo)
		if err != nil {
			return fmt.Errorf("[Error] Error generating mutation context: %w", err)
		}

		// 2. Analyze Mutation with context
		_, err = AnalyzeMutation(mutationContext)
		if err != nil {
			return fmt.Errorf("[Error] Error analyzing mutation: %v", err)
		}

		// 3. Save result
	}

	/*~*~*~*~*~*~*~*~*~*~* Post-conditions ~*~*~*~*~*~*~*~*~*~*~*/
	// Post-conditions
	// Panic:
	// - There should be some kind of file created with the results.
	// - The created result file shouldn't be empty

	return nil
}

func extractGambitResultsJSON(mutantsDirPath string) (map[string]MutantJSONInfo, error) {
	// Get the parent directory of the provided mutantsSpecificPath.
	// For "./gambit_out/mutants", parentDir will be "./gambit_out".
	parentDir := filepath.Dir(mutantsDirPath)

	gambitResultsPath := filepath.Join(parentDir, "gambit_results.json")

	if _, err := os.Stat(gambitResultsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("[Error] Path does not exist: %s", gambitResultsPath)
	} else if err != nil {
		return nil, fmt.Errorf("[Error] Error accessing path %s: %w", gambitResultsPath, err)
	}

	data, err := os.ReadFile(gambitResultsPath)
	if err != nil {
		return nil, fmt.Errorf("[Error] Error reading JSON %s: %w", gambitResultsPath, err)
	}

	var results []MutantJSONInfo
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("[Error] Failed to unmarshal gambit results JSON %s: %w", gambitResultsPath, err)
	}

	resultsMap := make(map[string]MutantJSONInfo)
	for _, r := range results {
		resultsMap[r.ID] = r
	}

	return resultsMap, nil
}

var mutationCommentRegex = regexp.MustCompile(`^\s*///.*Mutation\((.*?)\).*$`)

// generateMutationAnalysisContext attempts to find a mutation comment and extract surrounding lines.
func generateMutationAnalysisContext(
	mutantId string,
	mutantsBaseDir string,
	gambitResultsJSON MutantJSONInfo,
) (MutationAnalysisContext, error) {
	var ctx MutationAnalysisContext

	ctx.MutationDiff = gambitResultsJSON.Diff
	ctx.MutationType = gambitResultsJSON.Description

	ctx.MutatedFilePathUsed = filepath.Join(mutantsBaseDir, mutantId, gambitResultsJSON.Original)

	file, err := os.Open(ctx.MutatedFilePathUsed)
	if err != nil {
		return ctx, fmt.Errorf("[Error] Failed to open mutated file '%s': %w", ctx.MutatedFilePathUsed, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return ctx, fmt.Errorf("[Error] Failed reading mutated file '%s': %w", ctx.MutatedFilePathUsed, err)
	}

	if len(lines) == 0 {
		ctx.MutationContext = "[Error: Mutated file is empty]"
		// Return an error so the main loop can decide to skip this mutant
		return ctx, fmt.Errorf("mutated file '%s' is empty", ctx.MutatedFilePathUsed)
	}

	markerLineIndex := -1 // 0-indexed
	for i, line := range lines {
		if mutationCommentRegex.MatchString(line) {
			markerLineIndex = i
			break // Take the first marker found
		}
	}

	if markerLineIndex != -1 {
		ctx.MutationMarkerLine = markerLineIndex + 1 // Store 1-indexed
	} else {
		ctx.MutationMarkerLine = 0 // Marker not found
		ctx.MutationContext = ""
		return ctx, fmt.Errorf("[Error] Mutation marker comment not found in '%s' using regex.\n", ctx.MutatedFilePathUsed)
	}

	linesBefore := 25
	linesAfter := 5

	start := max(markerLineIndex-linesBefore, 0)

	end := min(markerLineIndex+linesAfter+1, len(lines))

	// Ensure 'start' is not greater than 'end', which can happen for very small files
	// or if markerLineIndex is near the beginning/end with large before/after counts.
	if start >= end {
		if len(lines) > 0 {
			// If there are lines, but the range is bad
			// Fallback to providing the whole file if the calculated slice is invalid
			// or if markerLineIndex was a guess (e.g., middle of file) resulting in a weird range.
			ctx.MutationContext += strings.Join(lines, "\n")
			fmt.Printf("[Info] Context range was invalid for '%s', providing whole file. Start: %d, End: %d, MarkerIdx: %d, len(lines): %d\n", ctx.MutatedFilePathUsed, start, end, markerLineIndex, len(lines))
		} else {
			ctx.MutationContext = ""
			return ctx, fmt.Errorf("[Error] Couldn't grab the context for '%s'. Start: %d, End: %d", ctx.MutatedFilePathUsed, start, end)
		}
	} else {
		ctx.MutationContext += strings.Join(lines[start:end], "\n")
	}

	return ctx, nil
}
