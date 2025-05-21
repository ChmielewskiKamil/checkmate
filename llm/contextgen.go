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
	"time"

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
	fmt.Println("[Info] LLM Analysis: Starting...")

	if analysisDb.LanguageModelProgress.MutantsProcessed == nil {
		analysisDb.LanguageModelProgress.MutantsProcessed = make(map[string]bool)
	}
	if analysisDb.AnalyzedFiles == nil {
		analysisDb.AnalyzedFiles = make(map[string]db.AnalyzedFile)
	}

	// --- Actions ---
	// 1. Get survivor IDs (directories present in mutantsDirPath are assumed to be survivors)
	var survivorIds []string
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

	// TODO: selectMutantsForLLMAnalysis will give a more accurate "to process" count.
	fmt.Printf("[Info] LLM Analysis: Found %d potential surviving mutants to analyze.\n", len(survivorIds))

	rand.Shuffle(len(survivorIds), func(i, j int) {
		survivorIds[i], survivorIds[j] = survivorIds[j], survivorIds[i]
	})

	// 2. Load gambit_results.json to get details for each survivor
	gambitMutantDetailsMap, err := extractGambitResultsJSON(mutantsDirPath)
	if err != nil {
		return fmt.Errorf("LLM Analysis: Could not extract details from gambit_results.json: %w", err)
	}

	// 3. Select mutants that need analysis or re-analysis
	mutantsToProcess := selectMutantsForLLMAnalysis(survivorIds, gambitMutantDetailsMap, analysisDb)
	if len(mutantsToProcess) == 0 {
		fmt.Println("[Info] LLM Analysis: No mutants require new or retried LLM analysis at this time.")
		return nil
	}
	fmt.Printf("[Info] LLM Analysis: Will attempt to analyze/re-analyze %d mutants.\n", len(mutantsToProcess))

	mutantsAnalyzedThisSession := 0

	for _, mutantID := range mutantsToProcess { // mutantID is the string like "1", "10", etc.
		mutantInfo, ok := gambitMutantDetailsMap[mutantID]
		if !ok {
			log.Printf("[Error] LLM Analysis: Consistency issue - no metadata for processing mutant ID %s. Skipping.\n", mutantID)
			continue
		}

		fmt.Printf("[Info] LLM Analysis: Preparing to analyze Mutant ID %s for original file '%s'.\n", mutantID, mutantInfo.Original)

		// Ensure AnalyzedFile entry and its sub-maps are correctly initialized
		originalFilePath := mutantInfo.Original
		fileData, fileExists := analysisDb.AnalyzedFiles[originalFilePath]
		if !fileExists {
			fileData = db.AnalyzedFile{
				FileSpecificStats:           db.FileSpecificStats{}, // Populated by init/slaying
				FileSpecificRecommendations: make([]string, 0),
				LLMAnalysisOutcomes:         make(map[string]db.MutantLLMAnalysisOutcome),
			}
		} else { // Entry exists, ensure sub-structures are not nil
			if fileData.FileSpecificRecommendations == nil {
				fileData.FileSpecificRecommendations = make([]string, 0)
			}
			if fileData.LLMAnalysisOutcomes == nil {
				fileData.LLMAnalysisOutcomes = make(map[string]db.MutantLLMAnalysisOutcome)
			}
		}

		// 4. Create context for the LLM
		llmContext, ctxErr := generateMutationAnalysisContext(mutantID, mutantsDirPath, mutantInfo)

		// TODO: Handle model used by an API call. For now hardcoded.
		modelUsedPlaceholder := "qwen2.5-coder-7b-instruct"

		outcome := db.MutantLLMAnalysisOutcome{
			MutantID:  mutantID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			ModelUsed: modelUsedPlaceholder,
		}

		if ctxErr != nil {
			log.Printf("[Error] LLM Analysis: Failed to generate LLM context for mutant ID %s: %v.\n", mutantID, ctxErr)
			outcome.Status = "FAILED_CONTEXT_GEN"
			outcome.ErrorMessage = ctxErr.Error()
			// LLMResponse remains empty (its zero value)
		} else {
			// 5. Analyze Mutation with context
			llmResponseContent, analysisErr := AnalyzeMutation(llmContext)

			if analysisErr != nil {
				log.Printf("[Error] LLM Analysis: LLM call failed for mutant ID %s: %v.\n", mutantID, analysisErr)
				outcome.Status = "FAILED_LLM_CALL"
				outcome.ErrorMessage = analysisErr.Error()
				// LLMResponse remains empty
			} else {
				fmt.Printf("\033[32m[Success] LLM Analysis: Successfully analyzed mutant ID %s.\033[0m\n", mutantID)
				outcome.Status = "COMPLETED"
				outcome.LLMResponse = llmResponseContent // Store raw LLM output

				fileData.FileSpecificRecommendations = append(fileData.FileSpecificRecommendations, llmResponseContent)
			}
		}

		fileData.LLMAnalysisOutcomes[mutantID] = outcome                   // Store detailed outcome
		analysisDb.AnalyzedFiles[originalFilePath] = fileData              // Update the map in dbState
		analysisDb.LanguageModelProgress.MutantsProcessed[mutantID] = true // Mark that an attempt outcome is now logged

		// 6. Periodic Save
		mutantsAnalyzedThisSession++
		// Adjust condition for last mutant: use len(mutantsToProcess)
		if mutantsAnalyzedThisSession%llmSaveInterval == 0 || mutantsAnalyzedThisSession == len(mutantsToProcess) {
			fmt.Printf("[Info] LLM Analysis: Saving progress to state file (%s)...\n", stateFilePath)
			if errSave := stateFileSaveFunc(stateFilePath, analysisDb); errSave != nil {
				// Changed from Printf to Errorf for consistency
				log.Printf("[Error] LLM Analysis: Failed to save state: %v\n", errSave)
			} else {
				fmt.Printf("\033[32m[Info] LLM Analysis: Progress saved.\033[0m\n")
			}
		}
	}

	fmt.Println("[Info] LLM Analysis session completed.")
	return nil
}

// Helper to select which mutants need analysis or re-analysis (from previous response)
// This function decides who goes into the main processing loop.
func selectMutantsForLLMAnalysis(
	allSurvivorIds []string,
	gambitDetailsMap map[string]MutantJSONInfo,
	analysisDb *db.MutationAnalysis,
) []string {
	var toProcess []string
	var skippedBecauseCompleted []string // For summary

	for _, mutantID := range allSurvivorIds {
		mutantInfo, ok := gambitDetailsMap[mutantID]
		if !ok {
			log.Printf("[Warning] LLM Selection: No details for survivor ID %s. Cannot determine original file. Skipping selection.", mutantID)
			continue
		}
		originalFilePath := mutantInfo.Original

		var existingOutcome db.MutantLLMAnalysisOutcome
		var outcomeExists bool

		if fileData, fileDataExists := analysisDb.AnalyzedFiles[originalFilePath]; fileDataExists {
			if fileData.LLMAnalysisOutcomes != nil { // Check if the map itself exists
				existingOutcome, outcomeExists = fileData.LLMAnalysisOutcomes[mutantID]
			}
		}

		if outcomeExists && existingOutcome.Status == "COMPLETED" {
			skippedBecauseCompleted = append(skippedBecauseCompleted, fmt.Sprintf("Mutant ID %s (%s)", mutantID, originalFilePath))
			continue // Already successfully completed
		}

		// If outcome doesn't exist, or status is not "COMPLETED" (e.g., FAILED_*, or a new PENDING_RETRY status),
		// then it needs to be processed.
		toProcess = append(toProcess, mutantID)
	}

	if len(skippedBecauseCompleted) > 0 {
		fmt.Printf("[Info] LLM Selection: Skipped %d mutant(s) already successfully analyzed by LLM:\n", len(skippedBecauseCompleted))
		// To avoid very long prints, summarize if many:
		if len(skippedBecauseCompleted) > 5 {
			for i := 0; i < 3; i++ {
				fmt.Printf("  - %s\n", skippedBecauseCompleted[i])
			}
			fmt.Printf("  ... and %d more.\n", len(skippedBecauseCompleted)-3)
		} else {
			for _, item := range skippedBecauseCompleted {
				fmt.Printf("  - %s\n", item)
			}
		}
	}
	return toProcess
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
