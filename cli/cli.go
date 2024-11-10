package cli

import (
	"bytes"
	"checkmate/assert"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Program struct {
	totalGeneratedMutants int32
	totalSlainMutants     int32
	perFileGenerated      map[string]int32
	perFileSlain          map[string]int32

	testCMD          *string // The command to run the test suite e.g. 'forge test'.
	mutantsDIR       *string // Path to the directory where generated mutants are stored.
	gambitConfigPath *string // Path to gambit's config json file
	skipGambit       *bool   // If you don't have or don't want to run gambit, skip it.
	contractsDIR     *string // Path to the folder where Solidity contracts are store. Default is "src/".
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

	return &p
}

func Run(p *Program) error {
	// Pre-conditions

	// Actions
	if !mutantsExist(p) && !*p.skipGambit {
		if !gambitConfigExists(p) {
			err := generateGambitConfig(p)
			if err != nil {
				return err
			}
			fmt.Println("\033[33m[Info] Generated gambit config successfuly.\n       Please review it and remove any files that you don't intend to test e.g. interfaces.\n       This will speed up the time it takes for gambit to generate the mutants and later\n       to run the analysis. After that re-run checkmate.\033[0m")
			os.Exit(0)
		}
		runGambit(p)
	}

	if !testSuitePasses(p) {
		return fmt.Errorf(`[Error] Your test suite fails the initial run.
        The test suite must be passing when the code is not mutated yet!
        Ensure that you have no failing tests before you attempt mutation testing your code.`)
	}

	saveMutationStats(p)

	testMutations(p)

	saveMutationStats(p)

	// Post-conditions
	return nil
}

func parseCmdFlags(p *Program) {
	testCMD := flag.String(
		"test-command",
		"forge test --fail-fast",
		"Specify the command to run your test suite. The default is 'forge test --fail-fast'.")

	mutantsDIR := flag.String(
		"mutants-dir",
		"./gambit_out/mutants",
		"Specify the path to the mutants directory. Default is './gambit_out/mutants'.")

	skipGambit := flag.Bool(
		"skip-gambit",
		false,
		"If you don't want checkmate to generate the mutants for you, specify this flag as 'true'.",
	)

	gambitConfigPath := flag.String(
		"config-path",
		"./gambit_config.json",
		"Specify the path to the gambit config json file. Default is './gambit_config.json'.",
	)

	contractFilesPath := flag.String(
		"contracts-path",
		"./src",
		"Specify the path to the folder with your smart contracts. The default is './src'.",
	)

	flag.Parse()

	p.testCMD = testCMD
	p.mutantsDIR = mutantsDIR
	p.skipGambit = skipGambit
	p.gambitConfigPath = gambitConfigPath
	p.contractsDIR = contractFilesPath

	// Post-conditions
	// TODO: Gambit config should be a valid json file
}

func mutantsExist(p *Program) bool {
	if _, err := os.Stat(*p.mutantsDIR); err != nil {
		fmt.Printf("[Info] Mutants directory at: '%s' does not exist.\n", *p.mutantsDIR)
		return false
	}

	entries, err := os.ReadDir(*p.mutantsDIR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Error] Could not read mutants directory: %v\n", err)
		return false
	}

	if len(entries) == 0 {
		fmt.Printf("[Info] Mutants directory at: '%s' is empty.\n", *p.mutantsDIR)
		return false
	}

	fmt.Printf("[Info] Found mutants directory at: '%s' and it is NOT empty.\n", *p.mutantsDIR)
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
	// Pre-conditions
	assert.PathExists(*p.gambitConfigPath)
	assert.NotEmpty(*p.gambitConfigPath)

	// Actions
	cmd := exec.Command("gambit", "mutate", "--json", *p.gambitConfigPath)
	err := cmd.Start()
	if err != nil {
		panic(fmt.Sprintf("[Error] Error starting gambit: %v", err))
	}

	fmt.Println("[Info] Mutating the code, please wait...")

	// Use Wait to block until the gambit is finished generating mutants.
	err = cmd.Wait()
	if err != nil {
		panic(fmt.Sprintf("[Error] Gambit finished with error: %v", err))
	}

	fmt.Println("[Info] Mutants generated ✅")

	// Post-conditions
	assert.NotEmpty(*p.mutantsDIR)
	// TODO: Assert that it contains at least 1 solidity file
}

func testSuitePasses(p *Program) bool {
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
				fmt.Fprintf(os.Stderr, "[Error] %s\n        Foundry's forge output:\n", err)
				fmt.Fprintln(os.Stderr, "\033[31m------- Foundry Error Zone - Start -------\033[0m")
				fmt.Fprintln(os.Stderr, string(output))
				fmt.Fprintln(os.Stderr, "\033[31m------- Foundry Error Zone - End -------\033[0m")
			}
		} else {
			fmt.Fprintf(os.Stderr, "[Error] There was an error running the command: %s\n", err)
		}
		return false
	}

	// If no errors, the test suite passed
	fmt.Println("[Info] Test suite passed successfully.")
	return true

	// Post-conditions
}

func saveMutationStats(p *Program) {
	// Pre-conditions
	assert.PathExists(*p.mutantsDIR)

	// Take snaphsot of the mutants in the mutants/ folder.
	// From here there are two possibilities:
	// 1. It is the first time we are doing this -> pre testing phase
	// 2. It is the second time -> post testing phase

	// Actions
	if p.totalGeneratedMutants == 0 {
		// TODO Assert that slain is 0 as well
		// TODO Assert that mutants/ dir is not empty and contains .sol files.
	}

	// Post-conditions
	// TODO Panic here, we should have returned earlier.
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

			gambitRemappings = append(gambitRemappings, fmt.Sprintf("%s=%s", from, to))
		}
	}

	// Post-conditions
	assert.True(len(gambitRemappings) > 0, "Transformed 0 gambit remappings.")

	return gambitRemappings
}

func testMutations(p *Program) {}
