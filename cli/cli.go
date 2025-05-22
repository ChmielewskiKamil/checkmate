package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/ChmielewskiKamil/checkmate/assert"
	"github.com/ChmielewskiKamil/checkmate/db"
	"github.com/ChmielewskiKamil/checkmate/llm"
)

const (
	stateFileName = "checkmate_analysis_state.json"
	saveInterval  = 10 // Save state every 10 mutants tested
)

type Program struct {
	testCMD          *string // The command to run the test suite e.g. 'forge test'.
	mutantsDIR       *string // Path to the directory where generated mutants are stored.
	gambitConfigPath *string // Path to gambit's config json file
	skipGambit       *bool   // If you don't have or don't want to run gambit, skip it.
	contractsDIR     *string // Path to the folder where Solidity contracts are store. Default is "src/".
	analyzeMutations *bool   // Whether to analyze mutations with LLM or not
	printReport      *bool   // Pretty print the mutation analysis report after all is done.

	// dbState holds all persistent information, loaded from and saved to mutationAnalysisStateFile.
	// All statistics and progress will be read from and written to this struct.
	dbState db.MutationAnalysis
}

type SolidityFile struct {
	Filename            string // Name of the file with the extension e.g 'Counter.sol'
	PathFromProjectRoot string // Path to the file from the project's root e.g. 'src/Counter.sol'
}

type GambitEntry struct {
	FilePath       string   `json:"filename"`        // File to the Solidity file from the project's root e.g. src/Counter.sol
	SolcRemappings []string `json:"solc_remappings"` // A list of Solc compiler remappings
}

func New() *Program {
	p := Program{}

	parseCmdFlags(&p)

	// Load existing state or initialize a new one
	loadedState, err := db.LoadStateFromFile(stateFileName)
	if err != nil {
		// This error means something went wrong beyond "file not found"
		// (e.g., corrupt JSON, permissions).
		log.Fatalf("[Critical] Error loading state from %s: %v. If the file is corrupt, please remove it to start fresh.", stateFileName, err)
	}
	p.dbState = loadedState // Assign loaded data (or fresh initialized struct if file didn't exist)

	return &p
}

func Run(p *Program) (err error) {
	var exitedForSpecialReason bool = false
	// Attempt to save state on exit, especially if an error occurs.
	// This is a best-effort save. A more robust solution might involve signal handling.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[CRITICAL] Panic occurred: %v. Attempting to save state...\033[0m\n", r)
			saveErr := db.SaveStateToFile(stateFileName, &p.dbState)
			if saveErr != nil {
				fmt.Fprintf(os.Stderr, "\033[31m[Error] Failed to save state during panic: %v\033[0m\n", saveErr)
			} else {
				fmt.Println("\033[32m[Info] State saved successfully during panic recovery.\033[0m")
			}
			panic(r) // Re-throw the panic
		}

		// If exited for --print or config-gen-only, don't show standard save messages.
		if exitedForSpecialReason {
			return
		}

		// 'err' is the named return value from Run().
		// The main function will be responsible for printing this 'err' to the user.
		// This defer will log its own actions regarding state saving.
		actionMessage := "final state"
		if err != nil { // If Run is returning an error
			actionMessage = "state due to an error in Run()"
		}

		fmt.Printf("[Info] Attempting to save %s...\n", actionMessage)

		saveErr := db.SaveStateToFile(stateFileName, &p.dbState)
		if saveErr != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[Error] Failed to save state to %s: %v\033[0m\n", stateFileName, saveErr)
			if err == nil {
				err = fmt.Errorf("failed to save final state: %w", saveErr)
			}
		} else {
			if err == nil {
				fmt.Println("\033[32m[Info] Final state saved successfully.\033[0m")
			} else {
				fmt.Println("\033[33m[Info] State (partially) saved despite earlier errors in Run().\033[0m")
			}
		}
	}()

	// --- Print Report Mode ---
	if *p.printReport {
		if p.dbState.OverallStats.MutantsTotalGenerated == 0 && len(p.dbState.AnalyzedFiles) == 0 {
			fmt.Println("\033[33m[Warning] No analysis data found in state file. Nothing to print.\033[0m")
			fmt.Printf("[Info] State file used: %s\n", stateFileName)
			return nil
		}
		fmt.Println("--- Checkmate Analysis Report ---")
		printMutationStatsReport(p)
		printLLMRecommendationsReport(p)
		printLLMAnalysisErrorsReport(p)
		fmt.Println("--- End of Report ---")
		exitedForSpecialReason = true
		return nil // Exit successfully after printing
	}

	// ---- LLM Analysis Mode ----
	if *p.analyzeMutations {
		fmt.Println("[Info] LLM Analysis mode selected.")

		llmErr := llm.AnalyzeMutations(
			*p.mutantsDIR,      // Path to the mutants directory (e.g., ./gambit_out/mutants)
			&p.dbState,         // Pointer to the persistent state object
			db.SaveStateToFile, // The actual save function
			stateFileName,      // The name of the state file
		)
		if llmErr != nil {
			return fmt.Errorf("LLM analysis failed: %w", llmErr) // Propagate error
		}

		fmt.Println("[Info] LLM Analysis completed.")
		return nil // LLM analysis mode finishes here
	}

	// Actions
	// ---- Slaying Mode ----
	gambitWasRunThisSession := false
	if !mutantsExist(p) && !*p.skipGambit {
		if !gambitConfigExists(p) {
			err := generateGambitConfig(p)
			if err != nil {
				return err
			}
			fmt.Println("\033[33m[Info] Generated gambit config successfuly.\n       Please review it and remove any files that you don't intend to test e.g. interfaces.\n       This will speed up the time it takes for gambit to generate the mutants and later\n       to run the analysis. After that re-run checkmate.\033[0m")
			exitedForSpecialReason = true
			return nil
		}

		// TODO: Before running Gambit ensure that the Solidity compiler version is
		// set to correct version.

		runGambit(p) // This generates mutants
		gambitWasRunThisSession = true
	}

	initializeGeneratedMutantStats(p)

	if p.dbState.OverallStats.MutantsTotalGenerated > 0 || gambitWasRunThisSession {
		fmt.Println("[Info] Saving baseline mutant statistics...")
		saveErr := db.SaveStateToFile(stateFileName, &p.dbState)
		if saveErr != nil {
			// Log as a warning, as the main analysis can often still proceed.
			log.Printf("[Warning] Failed to save baseline state after populating/refreshing stats: %v\n", saveErr)
		} else {
			fmt.Println("[Info] Baseline mutant statistics saved successfully.")
		}
	}

	fmt.Printf("[Info] Loaded analysis state from %s. Overall Mutants Generated: %d\n",
		stateFileName, p.dbState.OverallStats.MutantsTotalGenerated)

	fmt.Println("[Info] Attempting an initial test run to check if your test suite is ready for the mutation analysis.")
	if !testSuitePasses(p, true) {
		return fmt.Errorf(`Your test suite fails the initial run.
        The test suite must be passing when the code is not mutated yet!
        Ensure that you have no failing tests before you attempt mutation testing your code.`)
	}

	printMutationStats(p)

	testErr := testMutations(p)
	if testErr != nil {
		return fmt.Errorf("Testing mutations failed: %w", testErr)
	}

	printMutationStats(p)
	fmt.Println("[Info] Mutation analysis completed.")

	// Post-conditions

	return nil
}

func initializeGeneratedMutantStats(p *Program) {
	fmt.Println("[Info] Initializing mutant stats in persistent state...")

	mutants := listSolidityFiles(*p.mutantsDIR)

	if len(mutants) == 0 && p.dbState.OverallStats.MutantsTotalGenerated == 0 {
		fmt.Println("\033[33m[Warning] No mutants found in directory and no prior state. Nothing to initialize.\033[0m")
		return
	}

	// Only initialize if not already meaningfully populated (e.g. from a previous run)
	if p.dbState.OverallStats.MutantsTotalGenerated == 0 {
		p.dbState.OverallStats.MutantsTotalGenerated = int32(len(mutants))
		// Reset other overall stats for a fresh count based on newly listed mutants
		p.dbState.OverallStats.MutantsTotalSlain = 0
		// Initially all generated are unslain until tested
		p.dbState.OverallStats.MutantsTotalUnslain = 0
		p.dbState.OverallStats.MutationScore = 0.0

		// Initialize per-file generated counts
		if p.dbState.AnalyzedFiles == nil {
			p.dbState.AnalyzedFiles = make(map[string]db.AnalyzedFile)
		}

		tempPerFileGenerated := make(map[string]int32)

		for _, mutant := range mutants {
			// Gambit usually outputs mutants in subdirs like "mutants/1/src/Contract.sol"
			// We need to get the original contract path that this mutant belongs to.
			// This requires parsing the mutant path or having this info from gambit_results.json.
			// TODO: This could try to see if gambit_result.json is available. If not
			// fall back to this mutant dir parsing logic.

			// Path like src/Contract.sol
			originalFilePath := getOriginalFilePathFromMutantPath(mutant.PathFromProjectRoot, *p.mutantsDIR)
			if originalFilePath == "" {
				log.Printf("[Warning] Could not determine original file path for mutant %s", mutant.PathFromProjectRoot)
				continue
			}

			tempPerFileGenerated[originalFilePath]++
		}

		for path, count := range tempPerFileGenerated {
			entry, ok := p.dbState.AnalyzedFiles[path]
			if !ok {
				entry = db.AnalyzedFile{
					FileSpecificStats:           db.FileSpecificStats{},
					FileSpecificRecommendations: make([]string, 0),
					LLMAnalysisOutcomes:         make(map[string]db.MutantLLMAnalysisOutcome),
				}
			} else {
				// If entry exists from loaded state, ensure sub-maps/slices are not nil
				if entry.FileSpecificRecommendations == nil {
					entry.FileSpecificRecommendations = make([]string, 0)
				}
				if entry.LLMAnalysisOutcomes == nil {
					entry.LLMAnalysisOutcomes = make(map[string]db.MutantLLMAnalysisOutcome)
				}
			}
			entry.FileSpecificStats.MutantsTotalGenerated = count
			entry.FileSpecificStats.MutantsTotalSlain = 0   // Reset for new count
			entry.FileSpecificStats.MutantsTotalUnslain = 0 // Reset
			entry.FileSpecificStats.MutationScore = 0.0
			p.dbState.AnalyzedFiles[path] = entry
		}
		fmt.Printf("[Info] Initialized stats for %d generated mutants across %d files.\n",
			p.dbState.OverallStats.MutantsTotalGenerated, len(p.dbState.AnalyzedFiles))
	}
}

func getOriginalFilePathFromMutantPath(mutantPath, mutantsBaseDir string) string {
	// Example: mutantPath = "gambit_out/mutants/15/src/MyContract.sol"
	//          contractsDir = "src"
	//          mutantsBaseDir = "gambit_out/mutants"
	// We want to extract "src/MyContract.sol"

	// Clean paths
	mutantsBaseDirClean := filepath.Clean(mutantsBaseDir) + string(filepath.Separator) // e.g., "gambit_out/mutants/"

	if strings.HasPrefix(mutantPath, mutantsBaseDirClean) {
		// Remaining path: "15/src/MyContract.sol"
		relativePathWithMutantId := strings.TrimPrefix(mutantPath, mutantsBaseDirClean)
		// Find the second separator to skip the mutant ID part
		parts := strings.SplitN(relativePathWithMutantId, string(filepath.Separator), 2)
		if len(parts) == 2 {
			return parts[1] // Should be "src/MyContract.sol"
		}
	}
	log.Printf("[Debug] Failed to parse original path from mutant path: %s", mutantPath)
	return "" // Or handle error appropriately
}

func parseCmdFlags(p *Program) {
	versionFlag := flag.Bool("version", false, "Print the checkmate version and exit (only works if you installed from GitHub via 'go install').")

	testCMD := flag.String(
		"test-command",
		"forge test --fail-fast",
		"Specify the command to run your test suite. For hardhat repos, you can use 'npx hardhat test --bail'.")

	mutantsDIR := flag.String(
		"mutants-dir",
		"./gambit_out/mutants",
		"Specify the path to the mutants directory.")

	skipGambit := flag.Bool(
		"skip-gambit",
		false,
		"If you don't want checkmate to generate the mutants for you, specify this flag as 'true'. In this mode it will just run the test suite over the previously generated mutants.",
	)

	gambitConfigPath := flag.String(
		"config-path",
		"./gambit_config.json",
		"Specify the path to the gambit config json file.",
	)

	contractFilesPath := flag.String(
		"contracts-path",
		"./src",
		"Specify the path to the folder with your smart contracts. For hardhat repositories this is usually './contracts'.",
	)

	analyzeMutations := flag.Bool(
		"analyze",
		false,
		"Analyze the mutations present in the gambit_out/mutants/ directory with the help of an LLM.",
	)

	printReport := flag.Bool("print", false, "Print a summary report from the last analysis state and exit.")

	flag.Parse()

	if *versionFlag {
		info, ok := debug.ReadBuildInfo()
		if ok {
			version := info.Main.Version
			if version == "(devel)" || version == "" {
				fmt.Println("dev (not built from tagged release)")
			} else {
				fmt.Println(version)
			}
		} else {
			fmt.Println("Unknown version. You might be building locally. Version will show if you downloaded via `go install`.")
		}
		os.Exit(0)
	}

	p.testCMD = testCMD
	p.mutantsDIR = mutantsDIR
	p.skipGambit = skipGambit
	p.gambitConfigPath = gambitConfigPath
	p.contractsDIR = contractFilesPath
	p.analyzeMutations = analyzeMutations
	p.printReport = printReport

	// Post-conditions
	// TODO: Gambit config should be a valid json file
}

func mutantsExist(p *Program) bool {
	if _, err := os.Stat(*p.mutantsDIR); err != nil {
		fmt.Printf("[Info] Mutants directory at: '%s' does not exist.\n", *p.mutantsDIR)
		return false
	}

	if len(listSolidityFiles(*p.mutantsDIR)) == 0 {
		fmt.Printf("[Info] The mutants directory at: '%s' exists but it DOES NOT contain any Solidity files.\n", *p.mutantsDIR)
		return false
	}

	fmt.Printf("[Info] Found mutants directory at: '%s' that contains Solidity files.\n", *p.mutantsDIR)
	return true
}

func gambitConfigExists(p *Program) bool {
	if info, err := os.Stat(*p.gambitConfigPath); os.IsNotExist(err) {
		fmt.Printf("[Info] Gambit config json file does not exist.\n")
		return false
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "[Error] There was a problem accessing the config path: %s.\n", err)
		return false
	} else if info.Size() == 0 {
		fmt.Fprintf(os.Stderr, "[Error] The provided gambit config file: %s is empty.\n", *p.gambitConfigPath)
		err = os.Remove(*p.gambitConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Error] Couldn't remove the empty config file: %s .\n", err)
		}
		return false
	}

	fmt.Printf("[Info] Gambit config found at: %s.\n", *p.gambitConfigPath)
	return true
}

func generateGambitConfig(p *Program) error {
	// Pre-conditions
	assert.PathNotExists(*p.gambitConfigPath)
	assert.PathExists(*p.contractsDIR)

	// Actions
	solidityFiles := listSolidityFiles(*p.contractsDIR)
	gambitEntries, err := generateGambitEntries(solidityFiles)
	if err != nil {
		return fmt.Errorf("[Error] Couldn't generate gambit entires: %s", err)
	}

	jsonData, err := json.MarshalIndent(gambitEntries, "", "    ")
	if err != nil {
		return fmt.Errorf("[Error] There was a problem marshalling gambit entries: %s", err)
	}

	file, err := os.Create(*p.gambitConfigPath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	// Post-conditions
	assert.PathExists(*p.gambitConfigPath)
	assert.NotEmpty(*p.gambitConfigPath)
	return nil
}

func runGambit(p *Program) {
	assert.PathExists(*p.gambitConfigPath)
	assert.NotEmpty(*p.gambitConfigPath)

	cmd := exec.Command("gambit", "mutate", "--json", *p.gambitConfigPath)

	stderrPipe, _ := cmd.StderrPipe()

	// Buffer to capture stderr to detect errors later
	stderrScanner := bufio.NewScanner(stderrPipe)

	errDetected := make(chan struct{})
	var snippet []string
	const snippetLines = 5 // length of solc compiler version mismatch error
	linesAfterMatch := 0
	var errorPrinted bool

	// Start a goroutine to scan stderr and capture the error
	go func() {
		for stderrScanner.Scan() {
			line := stderrScanner.Text()

			// If we detect an error in stderr
			if strings.Contains(line, "compiler returned exit code") ||
				strings.Contains(line, "Source file requires different compiler version") {

				// Collect the error snippet
				if !errorPrinted {
					// If it's the first match, print the error and collect further lines
					snippet = append(snippet, line)
					linesAfterMatch = 0
					errorPrinted = true
				} else {
					// Continue collecting the error lines
					snippet = append(snippet, line)
				}
			} else if len(snippet) > 0 && linesAfterMatch < snippetLines {
				// Collect the following lines for the snippet
				snippet = append(snippet, line)
				linesAfterMatch++
				if linesAfterMatch >= snippetLines {
					// Stop reading and signal fatal error
					errDetected <- struct{}{}
					return
				}
			}
		}
	}()

	// Start the process
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[91m[Error] Failed to start gambit: %v\033[0m\n", err)
		os.Exit(1)
	}

	fmt.Printf("\033[92m[Info] Mutating the code with gambit, please wait...\n       This might take a while for bigger projects (e.g. over 15 minutes).\033[0m\n")

	// Use select block to either handle the error or continue execution
	select {
	case <-errDetected:
		// Handle error when detected
		fmt.Fprintln(os.Stderr, "\n\033[91m[Error] Solidity compilation failed during mutation.\033[0m")
		fmt.Fprintln(os.Stderr, "\033[93m[Hint] Your local Solidity compiler (solc) version may not match the pragma version declared in your contracts.\033[0m")
		fmt.Fprintln(os.Stderr, "\033[93m[Hint] You can install and switch compiler versions using solc-select:\033[0m")
		fmt.Fprintln(os.Stderr, "\033[93m    pip install solc-select\033[0m")
		fmt.Fprintln(os.Stderr, "\033[93m    solc-select install 0.8.23   # change to correct version\033[0m")
		fmt.Fprintln(os.Stderr, "\033[93m    solc-select use 0.8.23       # change to correct version\033[0m")
		fmt.Fprintln(os.Stderr, "\n\033[91m[Error] The compiler error snippet is shown below: \033[0m")
		for _, l := range snippet {
			fmt.Fprintln(os.Stderr, l)
		}

		_ = cmd.Process.Kill()
		os.Exit(1)

	case err := <-waitForCmd(cmd):
		if err != nil {
			// Handle Gambit process exit error
			fmt.Fprintf(os.Stderr, "\033[91m[Error] Gambit exited with error: %v\033[0m\n", err)
			os.Exit(1)
		}
	}

	// If no error detected, print the success message
	fmt.Println("\n[Info] Mutants generated ✅")
	assert.NotEmpty(*p.mutantsDIR)
	assert.True(len(listSolidityFiles(*p.mutantsDIR)) > 0, "There are no Solidity files in the mutants directory after running 'gambit mutate'.")
}

// waitForCmd wraps cmd.Wait() so we can use it in a select block
func waitForCmd(cmd *exec.Cmd) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	return done
}

func testSuitePasses(p *Program, detailedLogs bool) bool {
	// Pre-conditions

	// sh -c enables the CMD to be passed as a single string without slicing
	cmd := exec.Command("sh", "-c", *p.testCMD)
	fmt.Printf("[Info] Running the test suite with: %s.\n", *p.testCMD)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if the command error is due to the command not being found
		if exitErr, ok := err.(*exec.ExitError); ok {
			// If exit code is non-zero, check if it's due to command not found or other reasons
			if exitErr.ExitCode() == 127 {
				fmt.Fprintf(os.Stderr, "[Error] Command not found: %s\n", *p.testCMD)
			} else {
				if detailedLogs {
					fmt.Fprintf(os.Stderr, "[Error] %s\n        Foundry's forge output:\n", err)
					fmt.Fprintln(os.Stderr, "\033[31m------- Foundry Error Zone - Start -------\033[0m")
					fmt.Fprintln(os.Stderr, string(output))
					fmt.Fprintln(os.Stderr, "\033[31m------- Foundry Error Zone - End -------\033[0m")
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "[Error] There was an error running the command: %s\n", err)
		}

		fmt.Println("[Info] Test suite failed.")
		return false
	}

	// If no errors, the test suite passed
	fmt.Println("[Info] Test suite passed successfully.")
	return true

	// Post-conditions
}

func printMutationStats(p *Program) {
	stats := p.dbState.OverallStats
	analyzedFiles := p.dbState.AnalyzedFiles

	fmt.Printf("\n--------- Mutation Stats - Start ---------\n\n")

	fmt.Printf("Total mutants generated: %d\n", stats.MutantsTotalGenerated)
	fmt.Printf("Total mutants unslain: %d\n", stats.MutantsTotalUnslain)
	fmt.Printf("Total mutants slain: %d\n", stats.MutantsTotalSlain)
	fmt.Printf("Overall Mutation Score: %.2f%%\n\n", stats.MutationScore)

	if len(analyzedFiles) > 0 {
		fmt.Printf("Below is the per file breakdown: \n")
		// Need to iterate in a sorted order for consistent output if possible, or just range
		for filePath, fileData := range analyzedFiles {
			fmt.Printf("File: %s\n", filePath)
			fmt.Printf("  Generated: %d\n", fileData.FileSpecificStats.MutantsTotalGenerated)
			unslainInFile := fileData.FileSpecificStats.MutantsTotalGenerated - fileData.FileSpecificStats.MutantsTotalSlain
			fmt.Printf("  Unslain:   %d\n", unslainInFile) // Calculate derived value
			fmt.Printf("  Slain:     %d\n", fileData.FileSpecificStats.MutantsTotalSlain)
			scoreInFile := float32(0.0)
			if fileData.FileSpecificStats.MutantsTotalGenerated > 0 {
				scoreInFile = (float32(fileData.FileSpecificStats.MutantsTotalSlain) / float32(fileData.FileSpecificStats.MutantsTotalGenerated)) * 100
			}
			fmt.Printf("  Score:     %.2f%%\n", scoreInFile) // Calculate derived value
			if len(fileData.FileSpecificRecommendations) > 0 {
				fmt.Printf("  LLM Recommendations:\n")
				for _, rec := range fileData.FileSpecificRecommendations {
					fmt.Printf("    - %s\n", rec)
				}
				fmt.Println()
			}
		}
	} else {
		fmt.Println("No per-file data available yet.")
	}

	fmt.Printf("\n--------- Mutation Stats - End -----------\n\n")
}

func listSolidityFiles(pathToContracts string) []SolidityFile {
	var solidityFiles []SolidityFile

	// Use an anonymous function to wrap the call to visitSolFile
	filepath.Walk(pathToContracts, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Error] There was a problem accessing path %q: %v\n", path, err)
			return err // Return any error to filepath.Walk
		}

		fileRecord := visitSolFile(path, info)
		if fileRecord != nil {
			solidityFiles = append(solidityFiles, *fileRecord)
		}

		return nil // Continue walking the directory
	})

	return solidityFiles
}

func visitSolFile(absolutePath string, info os.FileInfo) *SolidityFile {
	if !info.IsDir() && strings.HasSuffix(info.Name(), ".sol") {
		// Compute the relative path based on absolute and project root.
		// E.g. given /user/projects/StakingProtocol/src/Pool.sol and project root ('./')
		// we will get /src/Pool.sol
		pathFromProjectRoot, err := filepath.Rel("./", absolutePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Error] Error computing relative path to file: %s", err)
			return nil
		}

		return &SolidityFile{
			Filename:            info.Name(),
			PathFromProjectRoot: pathFromProjectRoot,
		}
	}

	return nil
}

func generateGambitEntries(solidityFiles []SolidityFile) ([]GambitEntry, error) {
	// Pre-condition
	assert.True(len(solidityFiles) > 0, "No Solidity files were provided. Can't generate gambit entries.")

	forgeRemappings, err := getForgeRemappings()
	if err != nil {
		return nil, err
	}

	gambitRemappings := transformForgeRemappings(forgeRemappings)

	assert.True(len(gambitRemappings) > 0, "Got 0 gambit remappings.")

	var gambitEntries []GambitEntry

	for _, file := range solidityFiles {
		entry := GambitEntry{
			FilePath:       file.PathFromProjectRoot,
			SolcRemappings: gambitRemappings,
		}

		gambitEntries = append(gambitEntries, entry)
	}

	// Post-condition
	assert.True(len(gambitEntries) > 0, "Generated 0 gambit entries.")

	return gambitEntries, nil
}

func getForgeRemappings() (string, error) {
	var out bytes.Buffer

	cmd := exec.Command("forge", "remappings")
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("Failed to run forge remappings: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("Forge remappings finished with an error: %v", err)
	}

	// Post-conditions
	// There is always at least: forge-std/=lib/forge-std/src/
	assert.True(out.Len() > 0, "Forge remappings are empty.")

	return out.String(), nil
}

func transformForgeRemappings(forgeRemappings string) []string {
	// Pre-conditions
	assert.True(len(forgeRemappings) > 0, "Cannot create gambit remappings if forge remappings are empty.")

	var gambitRemappings []string

	lines := strings.Split(forgeRemappings, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split each remapping into "from" and "to" parts
		parts := strings.Split(line, "/=")
		if len(parts) == 2 {
			from := strings.TrimSpace(parts[0])
			to := strings.TrimSpace(parts[1])

			if checkRemappingExists(to) {
				gambitRemappings = append(gambitRemappings, fmt.Sprintf("%s=%s", from, to))
			}
		}
	}

	// Post-conditions
	assert.True(len(gambitRemappings) > 0, "Transformed 0 gambit remappings.")

	return gambitRemappings
}

func checkRemappingExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		// Path doesn't exist
		return false
	}

	// Remapping points to a dir
	return info.IsDir()
}

func testMutations(p *Program) error {
	// Pre-conditions
	assert.True(p.dbState.OverallStats.MutantsTotalGenerated > 0, "Can't perform analysis if there are no mutants.")

	// Ensure SlayingProgress map is initialized
	if p.dbState.SlayingProgress.MutantsProcessed == nil {
		p.dbState.SlayingProgress.MutantsProcessed = make(map[string]bool)
	}

	fmt.Printf("\n\033[32m[Info] Starting the mutation analysis.\033[0m\n\n")

	mutantFiles := listSolidityFiles(*p.mutantsDIR)
	mutantsProcessedCount := 0
	consecutiveSkippedCount := 0 // Counter for consecutively skipped mutants

	for _, mutantFile := range mutantFiles {
		mutantIdentifier := mutantFile.PathFromProjectRoot // Using path as an ID

		if p.dbState.SlayingProgress.MutantsProcessed[mutantIdentifier] {
			consecutiveSkippedCount++
			continue // Skip already processed mutants
		}

		// If we reach here, this mutant is not skipped.
		// If there were previously skipped mutants in a sequence, print a summary for them.
		if consecutiveSkippedCount > 0 {
			if consecutiveSkippedCount == 1 {
				fmt.Printf("[Info] Skipped 1 already processed mutant.\n")
			} else {
				fmt.Printf("[Info] Skipped %d already processed mutants.\n", consecutiveSkippedCount)
			}
			consecutiveSkippedCount = 0 // Reset for the next potential batch of skipped ones
		}

		originalFilePath := getOriginalFilePathFromMutantPath(mutantFile.PathFromProjectRoot, *p.mutantsDIR)
		if originalFilePath == "" {
			log.Printf("[Warning] Could not determine original file for mutant %s. Skipping.", mutantFile.PathFromProjectRoot)
			p.dbState.SlayingProgress.MutantsProcessed[mutantIdentifier] = true // Mark as processed to avoid re-attempt
			continue
		}

		// Ensure AnalyzedFile entry exists
		fileAnalysisEntry, ok := p.dbState.AnalyzedFiles[originalFilePath]
		if !ok {
			log.Printf("[Warning] No analysis entry for original file %s. Initializing.", originalFilePath)
			fileAnalysisEntry = db.AnalyzedFile{
				FileSpecificStats: db.FileSpecificStats{
					// MutantsTotalGenerated should have been set by initializeGeneratedMutantStats
					MutantsTotalGenerated: p.dbState.AnalyzedFiles[originalFilePath].FileSpecificStats.MutantsTotalGenerated,
				},
				FileSpecificRecommendations: []string{},
			}
		}

		destinationPath := originalFilePath // Path in the project to overwrite with mutant
		backupPath := destinationPath + ".bak"

		err := copyFile(destinationPath, backupPath)
		if err != nil {
			return fmt.Errorf("failed to backup original file %s: %w", destinationPath, err)
		}

		err = copyFile(mutantFile.PathFromProjectRoot, destinationPath)
		if err != nil {
			_ = os.Remove(backupPath) // Attempt cleanup
			return fmt.Errorf("failed to copy mutant %s to %s: %w", mutantFile.PathFromProjectRoot, destinationPath, err)
		}

		if !testSuitePasses(p, false) { // Test suite fails -> mutant is slain
			fmt.Printf("[Info] Mutant slain 🗡️ (%s)\n", mutantFile.PathFromProjectRoot)
			mutantDirToRemove := filepath.Dir(filepath.Dir(mutantFile.PathFromProjectRoot)) // Go up twice if structure is id/src/file.sol
			if strings.HasPrefix(mutantDirToRemove, filepath.Clean(*p.mutantsDIR)) {        // Basic safety check
				os.RemoveAll(mutantDirToRemove)
				// fmt.Printf("[Debug] Would remove mutant directory: %s\n", mutantDirToRemove)
			}

			p.dbState.OverallStats.MutantsTotalSlain++
			fileAnalysisEntry.FileSpecificStats.MutantsTotalSlain++
		} else {
			fmt.Printf("[Info] Test suite didn't catch the bug ❌ Mutant unslain: (%s)\n", mutantFile.PathFromProjectRoot)
			// Update total unslain as derivation of total generated and slain below.
		}

		// Restore original file
		err = copyFile(backupPath, destinationPath)
		if err != nil {
			return fmt.Errorf("failed to restore backup for %s: %w", destinationPath, err)
		}
		err = os.Remove(backupPath)
		if err != nil {
			return fmt.Errorf("failed to remove backup file %s: %w", backupPath, err)
		}

		// Update stats after test
		p.dbState.SlayingProgress.MutantsProcessed[mutantIdentifier] = true

		// Recalculate unslain counts and scores
		fileAnalysisEntry.FileSpecificStats.MutantsTotalUnslain = fileAnalysisEntry.FileSpecificStats.MutantsTotalGenerated - fileAnalysisEntry.FileSpecificStats.MutantsTotalSlain

		if fileAnalysisEntry.FileSpecificStats.MutantsTotalGenerated > 0 {
			fileAnalysisEntry.FileSpecificStats.MutationScore =
				(float32(fileAnalysisEntry.FileSpecificStats.MutantsTotalSlain) / float32(fileAnalysisEntry.FileSpecificStats.MutantsTotalGenerated)) * 100
		}

		p.dbState.AnalyzedFiles[originalFilePath] = fileAnalysisEntry

		mutantsProcessedCount++

		if mutantsProcessedCount%saveInterval == 0 {
			p.dbState.OverallStats.MutantsTotalUnslain = p.dbState.OverallStats.MutantsTotalGenerated - p.dbState.OverallStats.MutantsTotalSlain
			if p.dbState.OverallStats.MutantsTotalGenerated > 0 {
				p.dbState.OverallStats.MutationScore =
					(float32(p.dbState.OverallStats.MutantsTotalSlain) / float32(p.dbState.OverallStats.MutantsTotalGenerated)) * 100
			}
			if errSave := db.SaveStateToFile(stateFileName, &p.dbState); errSave != nil {
				log.Printf("[Warning] Failed to save state during testing mutations: %v", errSave)
			} else {

				fmt.Printf("\033[32m[Info] Progress saved after processing %d mutants.\033[0m\n", mutantsProcessedCount)
			}
		}
	}

	// Final calculation for overall unslain and score
	p.dbState.OverallStats.MutantsTotalUnslain = p.dbState.OverallStats.MutantsTotalGenerated - p.dbState.OverallStats.MutantsTotalSlain
	if p.dbState.OverallStats.MutantsTotalGenerated > 0 {
		p.dbState.OverallStats.MutationScore =
			(float32(p.dbState.OverallStats.MutantsTotalSlain) / float32(p.dbState.OverallStats.MutantsTotalGenerated)) * 100
	}

	return nil
}

func copyFile(src, dst string) error {
	assert.PathExists(src)

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// Renamed from printMutationStats to be more specific for report generation
func printMutationStatsReport(p *Program) {
	stats := p.dbState.OverallStats
	analyzedFiles := p.dbState.AnalyzedFiles

	fmt.Printf("\n## Mutation Analysis Statistics\n\n")

	fmt.Printf("### Overall Statistics\n")
	fmt.Printf("- Total mutants generated: %d\n", stats.MutantsTotalGenerated)
	// Ensure Unslain is calculated if not already up-to-date from a full run
	actualUnslain := stats.MutantsTotalGenerated - stats.MutantsTotalSlain
	fmt.Printf("- Total mutants unslain: %d\n", actualUnslain)
	fmt.Printf("- Total mutants slain: %d\n", stats.MutantsTotalSlain)
	fmt.Printf("- Overall Mutation Score: %.2f%%\n\n", stats.MutationScore)

	if len(analyzedFiles) > 0 {
		fmt.Printf("### Per-File Breakdown\n")
		// Sort file paths for consistent output
		sortedFilePaths := make([]string, 0, len(analyzedFiles))
		for k := range analyzedFiles {
			sortedFilePaths = append(sortedFilePaths, k)
		}
		sort.Strings(sortedFilePaths)

		for _, filePath := range sortedFilePaths {
			fileData := analyzedFiles[filePath]
			fmt.Printf("\n#### File: `%s`\n", filePath)
			fmt.Printf("- Generated: %d\n", fileData.FileSpecificStats.MutantsTotalGenerated)
			fileUnslain := fileData.FileSpecificStats.MutantsTotalGenerated - fileData.FileSpecificStats.MutantsTotalSlain
			fmt.Printf("- Unslain:   %d\n", fileUnslain)
			fmt.Printf("- Slain:     %d\n", fileData.FileSpecificStats.MutantsTotalSlain)
			fmt.Printf("- Score:     %.2f%%\n", fileData.FileSpecificStats.MutationScore)
		}
	} else if stats.MutantsTotalGenerated > 0 { // If overall stats exist but no per-file breakdown yet
		fmt.Println("No per-file breakdown available in the current state.")
	}
	fmt.Println()
}

func printLLMRecommendationsReport(p *Program) {
	analyzedFiles := p.dbState.AnalyzedFiles
	if len(analyzedFiles) == 0 {
		return // Nothing to print if no files were analyzed
	}

	fmt.Printf("\n## LLM Test Case Recommendations\n")
	foundRecommendations := false

	sortedFilePaths := make([]string, 0, len(analyzedFiles))
	for k := range analyzedFiles {
		sortedFilePaths = append(sortedFilePaths, k)
	}
	sort.Strings(sortedFilePaths)

	for _, filePath := range sortedFilePaths {
		fileData := analyzedFiles[filePath]
		if fileData.FileSpecificRecommendations != nil && len(fileData.FileSpecificRecommendations) > 0 {
			if !foundRecommendations {
				foundRecommendations = true
			}
			fmt.Printf("\n### File: `%s`\n", filePath)
			for _, rec := range fileData.FileSpecificRecommendations {
				// Assuming rec is the raw LLM output
				fmt.Printf("- %s\n", rec) // Markdown list item
			}
		}
	}

	if !foundRecommendations {
		fmt.Println("No LLM recommendations found in the current analysis state.")
	}
	fmt.Println()
}

func printLLMAnalysisErrorsReport(p *Program) {
	analyzedFiles := p.dbState.AnalyzedFiles
	if len(analyzedFiles) == 0 {
		return
	}

	fmt.Printf("\n## LLM Analysis Issues Encountered\n")
	foundErrors := false

	sortedFilePaths := make([]string, 0, len(analyzedFiles))
	for k := range analyzedFiles {
		sortedFilePaths = append(sortedFilePaths, k)
	}
	sort.Strings(sortedFilePaths)

	for _, filePath := range sortedFilePaths {
		fileData := analyzedFiles[filePath]
		if fileData.LLMAnalysisOutcomes != nil && len(fileData.LLMAnalysisOutcomes) > 0 {
			var fileErrors []string
			// Sort mutant IDs for consistent error reporting order
			mutantIDsWithErrors := make([]string, 0, len(fileData.LLMAnalysisOutcomes))
			for mutantID := range fileData.LLMAnalysisOutcomes {
				mutantIDsWithErrors = append(mutantIDsWithErrors, mutantID)
			}
			sort.Strings(mutantIDsWithErrors)

			for _, mutantID := range mutantIDsWithErrors {
				outcome := fileData.LLMAnalysisOutcomes[mutantID]
				if outcome.Status != "COMPLETED" && outcome.ErrorMessage != "" {
					fileErrors = append(fileErrors, fmt.Sprintf("  - Mutant ID `%s`: %s (Status: %s, Timestamp: %s)",
						mutantID, outcome.ErrorMessage, outcome.Status, outcome.Timestamp))
				}
			}

			if len(fileErrors) > 0 {
				if !foundErrors {
					foundErrors = true
				}
				fmt.Printf("\n### File: `%s`\n", filePath)
				for _, errMsg := range fileErrors {
					fmt.Println(errMsg)
				}
			}
		}
	}

	if !foundErrors {
		fmt.Println("No LLM analysis errors or issues recorded.")
	}
	fmt.Println()
}
