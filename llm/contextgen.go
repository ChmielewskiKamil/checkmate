package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ChmielewskiKamil/checkmate/db"
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

// func AnalyzeMutations(mutantsDirPath string) error {
// 	/*~*~*~*~*~*~*~*~*~*~*~* Pre-conditions ~*~*~*~*~*~*~*~*~*~*~*/
// 	// Handle errors for users:
// 	// - If the mutations directory is nonexistant, return error.
// 	// - If there are no mutations in the mutations directory, return error.
// 	// Panic:
// 	// - Since this is the entrypoint function, there are no strong
// 	// preconditions that should cause a panic.
// 	info, err := os.Stat(mutantsDirPath)
// 	if os.IsNotExist(err) {
// 		return fmt.Errorf("[Error] The %s path does not exist. Before running the LLM analysis, you might want to run checkmate in standard mode to generate and slay the mutants first.", mutantsDirPath)
// 	} else if err != nil {
// 		return fmt.Errorf("[Error] Error accessing path %s: %w", mutantsDirPath, err)
// 	}
//
// 	if !info.IsDir() {
// 		return fmt.Errorf("[Error] The %s path exists, but is not a directory.", mutantsDirPath)
// 	}
//
// 	entries, err := os.ReadDir(mutantsDirPath)
// 	if err != nil {
// 		return fmt.Errorf("[Error] There was a problem reading directory: %v", err)
// 	}
//
// 	if len(entries) == 0 {
// 		return fmt.Errorf("[Error] The directory is empty: %s", mutantsDirPath)
// 	}
//
// 	// TODO: Check if particular mutant file is not empty (?)
//
// 	/*~*~*~*~*~*~*~*~*~*~*~*~* Actions ~*~*~*~*~*~*~*~*~*~*~*~*~*/
//
// 	var survivorIds []string
// 	for _, entry := range entries {
// 		if entry.IsDir() {
// 			survivorIds = append(survivorIds, entry.Name())
// 		}
// 	}
//
// 	// TODO: This can be removed for production build but helps with picking
// 	// mutations to analyze at random.
// 	rand.Shuffle(len(survivorIds), func(i, j int) {
// 		survivorIds[i], survivorIds[j] = survivorIds[j], survivorIds[i]
// 	})
//
// 	// From gambitSurvivorsMap we can extract the Mutation Type and Mutation
// 	// Diff.
// 	gambitSurvivorsMap, err := extractGambitResultsJSON(mutantsDirPath)
// 	if err != nil {
// 		return fmt.Errorf("[Error] Error extracting mutation results from gambit_results.json: %w", err)
// 	}
//
// 	for _, id := range survivorIds {
// 		jsonInfo, ok := gambitSurvivorsMap[id]
// 		if !ok {
// 			fmt.Printf("[Warning] No JSON entry found for mutant survivor with ID %s. Skipping.\n", id)
// 			continue
// 		}
// 		// 1. Create context
// 		mutationContext, err := generateMutationAnalysisContext(id, mutantsDirPath, jsonInfo)
// 		if err != nil {
// 			return fmt.Errorf("[Error] Error generating mutation context: %w", err)
// 		}
//
// 		// 2. Analyze Mutation with context
// 		_, err = AnalyzeMutation(mutationContext)
// 		if err != nil {
// 			return fmt.Errorf("[Error] Error analyzing mutation: %v", err)
// 		}
//
// 		// 3. Save result
// 	}
//
// 	/*~*~*~*~*~*~*~*~*~*~* Post-conditions ~*~*~*~*~*~*~*~*~*~*~*/
// 	// Post-conditions
// 	// Panic:
// 	// - There should be some kind of file created with the results.
// 	// - The created result file shouldn't be empty
//
// 	return nil
// }

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

const llmSaveInterval = 3 // Save progress every 3 LLM calls

// AnalyzeMutations orchestrates the LLM analysis of surviving mutants and updates dbState.
func AnalyzeMutations(
	mutantsDirPath string, // Path to the root of mutant directories (e.g., "./gambit_out/mutants")
	analysisDb *db.MutationAnalysis,
	stateFileSaveFunc func(filePath string, data *db.MutationAnalysis) error, // Callback for saving state
	stateFilePath string, // Path to the state file (e.g., "checkmate_analysis_state.json")
) error {
	// --- Pre-condition checks for mutantsDirPath ---
	info, err := os.Stat(mutantsDirPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("LLM Analysis: The mutants directory %s does not exist. Ensure mutants are generated and slaying (if any) is done before LLM analysis.", mutantsDirPath)
	} else if err != nil {
		return fmt.Errorf("LLM Analysis: Error accessing mutants directory %s: %w", mutantsDirPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("LLM Analysis: The path %s is not a directory.", mutantsDirPath)
	}

	// --- Ensure progress maps in dbState are initialized ---
	if analysisDb.LanguageModelProgress.MutantsProcessed == nil {
		analysisDb.LanguageModelProgress.MutantsProcessed = make(map[string]bool)
	}
	if analysisDb.AnalyzedFiles == nil {
		analysisDb.AnalyzedFiles = make(map[string]db.AnalyzedFile)
	}

	// --- Actions ---
	// 1. Get survivor IDs (directories present in mutantsDirPath are assumed to be survivors)
	var survivorIds []string // These are the string IDs of the mutant subdirectories
	entries, err := os.ReadDir(mutantsDirPath)
	if err != nil {
		return fmt.Errorf("LLM Analysis: Error reading mutants directory %s: %w", mutantsDirPath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			survivorIds = append(survivorIds, entry.Name())
		}
	}

	if len(survivorIds) == 0 {
		fmt.Println("\033[33m[Warning] LLM Analysis: No surviving mutant directories found in", mutantsDirPath, "to analyze.\033[0m")
		return nil
	}
	fmt.Printf("[Info] LLM Analysis: Found %d potential surviving mutants to analyze.\n", len(survivorIds))

	// Optional: Shuffle for variety if re-running or for testing different mutants first
	rand.Shuffle(len(survivorIds), func(i, j int) {
		survivorIds[i], survivorIds[j] = survivorIds[j], survivorIds[i]
	})

	// 2. Load gambit_results.json to get details (like original file path) for each survivor
	gambitMutantDetailsMap, err := extractGambitResultsJSON(mutantsDirPath)
	if err != nil {
		return fmt.Errorf("LLM Analysis: Could not extract details from gambit_results.json: %w", err)
	}

	mutantsAnalyzedThisSession := 0
	skippedSummarizer := newSkipSummarizer("LLM Analysis") // Defined further down or in a utility file

	for _, mutantID := range survivorIds { // mutantID is the string like "1", "10", etc.
		mutantInfo, ok := gambitMutantDetailsMap[mutantID]
		if !ok {
			log.Printf("[Warning] LLM Analysis: No metadata found in gambit_results.json for mutant ID %s (directory name). Skipping.\n", mutantID)
			// Mark as processed to avoid re-attempting this ID if it's truly problematic
			analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] = true
			continue
		}

		// 3. Check if this mutant has already been processed by the LLM
		if analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] {
			skippedSummarizer.RecordSkip(fmt.Sprintf("Mutant ID %s (%s)", mutantID, mutantInfo.Original))
			continue
		}
		skippedSummarizer.PrintSummaryIfNeeded() // If a sequence of skips just ended

		fmt.Printf("[Info] LLM Analysis: Preparing to analyze Mutant ID %s for original file '%s'.\n", mutantID, mutantInfo.Original)

		// 4. Create context for the LLM
		llmContext, ctxErr := generateMutationAnalysisContext(mutantID, mutantsDirPath, mutantInfo)
		if ctxErr != nil {
			log.Printf("[Error] LLM Analysis: Failed to generate LLM context for mutant ID %s: %v. Skipping this mutant.\n", mutantID, ctxErr)
			analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] = true // Mark as attempted (context gen failed)
			recordLLMFailure(analysisDb, mutantInfo.Original, mutantID, "Context generation failed: "+ctxErr.Error())
			continue // Move to the next mutant
		}

		// 5. Analyze Mutation with context
		llmResponseContent, analysisErr := AnalyzeMutation(llmContext)

		if analysisErr != nil {
			log.Printf("[Error] LLM Analysis: LLM call failed for mutant ID %s: %v.\n", mutantID, analysisErr)
			analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] = true // Mark as attempted (LLM call failed)
			recordLLMFailure(analysisDb, mutantInfo.Original, mutantID, "LLM call failed: "+analysisErr.Error())
		} else {
			fmt.Printf("\033[32m[Success] LLM Analysis: Successfully analyzed mutant ID %s.\033[0m\n", mutantID)
			// Append the successful response to the recommendations for the original file
			originalFilePath := mutantInfo.Original
			fileData, fileExists := analysisDb.AnalyzedFiles[originalFilePath]
			if !fileExists {
				// This should ideally not happen if initializeGeneratedMutantStats ran based on gambit_results.json
				log.Printf("[Warning] LLM Analysis: No prior analysis entry for original file %s. Initializing it now.", originalFilePath)
				fileData = db.AnalyzedFile{
					FileSpecificStats:           db.FileSpecificStats{}, // Should have been set by slaying phase
					FileSpecificRecommendations: []string{},             // Initialize empty slice
				}
			}
			// Ensure the slice is not nil (important if loading from an older state file)
			if fileData.FileSpecificRecommendations == nil {
				fileData.FileSpecificRecommendations = []string{}
			}

			// Format the recommendation clearly
			recommendation := fmt.Sprintf("Suggestion for Mutant ID %s (Description: %s): %s",
				mutantID, mutantInfo.Description, llmResponseContent)
			fileData.FileSpecificRecommendations = append(fileData.FileSpecificRecommendations, recommendation)
			analysisDb.AnalyzedFiles[originalFilePath] = fileData // Update the map

			analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] = true // Mark as successfully processed
		}

		// 6. Periodic Save
		mutantsAnalyzedThisSession++
		if mutantsAnalyzedThisSession%llmSaveInterval == 0 || mutantsAnalyzedThisSession == len(survivorIds)-skippedSummarizer.TotalSkippedOverall() { // Adjust total for accurate end check
			fmt.Printf("[Info] LLM Analysis: Saving progress to state file (%s)...\n", stateFilePath)
			if errSave := stateFileSaveFunc(stateFilePath, analysisDb); errSave != nil {
				fmt.Printf("[Error] LLM Analysis: Failed to save state: %v\n", errSave)
			} else {
				fmt.Printf("\033[32m[Info] LLM Analysis: Progress saved.\033[0m\n")
			}
		}
	}
	skippedSummarizer.PrintSummaryIfNeededAndFinal() // Print any final batch of skips

	fmt.Println("[Info] LLM Analysis for all targeted survivors completed.")
	return nil
}

// Helper to record a generic LLM failure in FileSpecificRecommendations
func recordLLMFailure(analysisDb *db.MutationAnalysis, originalFilePath, mutantID, reason string) {
	fileData, fileExists := analysisDb.AnalyzedFiles[originalFilePath]
	if !fileExists {
		log.Printf("[Warning] LLM Failure Record: No prior analysis entry for original file %s. Initializing.", originalFilePath)
		fileData = db.AnalyzedFile{
			FileSpecificStats:           db.FileSpecificStats{},
			FileSpecificRecommendations: []string{},
		}
	}
	if fileData.FileSpecificRecommendations == nil {
		fileData.FileSpecificRecommendations = make([]string, 0)
	}
	failureMessage := fmt.Sprintf("Mutant ID %s: LLM analysis attempt failed - %s", mutantID, reason)
	fileData.FileSpecificRecommendations = append(fileData.FileSpecificRecommendations, failureMessage)
	analysisDb.AnalyzedFiles[originalFilePath] = fileData
}

// --- skipSummarizer utility (can be in a util.go file or here) ---
type skipSummarizer struct {
	moduleName          string
	consecutiveSkipped  int
	totalSkippedThisRun int // Total skipped since summarizer was created or last final print
}

func newSkipSummarizer(moduleName string) *skipSummarizer {
	return &skipSummarizer{moduleName: moduleName}
}

func (s *skipSummarizer) RecordSkip(itemName string) {
	s.consecutiveSkipped++
	s.totalSkippedThisRun++
	// To make it less verbose, we don't print the individual "skipping X" here.
	// It will be summarized when a non-skipped item is encountered or at the end.
}

func (s *skipSummarizer) PrintSummaryIfNeeded() {
	if s.consecutiveSkipped > 0 {
		logMsg := fmt.Sprintf("[Info] %s: Skipped %d already processed item(s).", s.moduleName, s.consecutiveSkipped)
		// Only print if we're about to process something new, so the summary makes sense.
		// This function is called *before* processing a new item.
		fmt.Println(logMsg)
		s.consecutiveSkipped = 0 // Reset for the next potential batch of skipped ones
	}
}

func (s *skipSummarizer) PrintSummaryIfNeededAndFinal() {
	s.PrintSummaryIfNeeded() // Print any pending consecutive skips
	// Optionally, print a grand total for the run if needed, though this might be too verbose.
	// if s.totalSkippedThisRun > 0 {
	// 	fmt.Printf("[Info] %s: In total, %d items were skipped during this phase as they were already processed.\n", s.moduleName, s.totalSkippedThisRun)
	// }
}

func (s *skipSummarizer) TotalSkippedOverall() int {
	return s.totalSkippedThisRun
}
