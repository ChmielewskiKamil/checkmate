package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ChmielewskiKamil/checkmate/assert"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type Program struct {
	totalGeneratedMutants int32
	totalUnslainMutants   int32
	perFileGenerated      map[string]int32
	perFileUnslain        map[string]int32

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

	perFileGenerated := make(map[string]int32)
	p.perFileGenerated = perFileGenerated

	perFileUnslain := make(map[string]int32)
	p.perFileUnslain = perFileUnslain

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

	fmt.Println("[Info] Attempting an initial test run to check if your test suite is ready for the mutation analysis.")
	if !testSuitePasses(p, true) {
		return fmt.Errorf(`[Error] Your test suite fails the initial run.
        The test suite must be passing when the code is not mutated yet!
        Ensure that you have no failing tests before you attempt mutation testing your code.`)
	}

	saveMutationStats(p)
	printMutationStats(p)

	err := testMutations(p)
	if err != nil {
		return err
	}

	saveMutationStats(p)
	printMutationStats(p)
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
	// Pre-conditions
	assert.PathExists(*p.gambitConfigPath)
	assert.NotEmpty(*p.gambitConfigPath)

	// Actions
	cmd := exec.Command("gambit", "mutate", "--json", *p.gambitConfigPath)

	// Report output streams in real time.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		panic(fmt.Sprintf("[Error] Error starting gambit: %v", err))
	}

	fmt.Println("[Info] Mutating the code with gambit, please wait...\n       This might take a while for bigger projects (e.g. over 15 minutes)")
	fmt.Println("")

	// Use Wait to block until the gambit is finished generating mutants.
	err = cmd.Wait()
	if err != nil {
		panic(fmt.Sprintf("\n[Error] Gambit finished with error: %v\n", err))
	}

	fmt.Println("\n[Info] Mutants generated ✅")

	// Post-conditions
	assert.NotEmpty(*p.mutantsDIR)
	assert.True(len(listSolidityFiles(*p.mutantsDIR)) > 0, "There are no Solidity files in the mutants directory after running 'gambit mutate'.")
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

func saveMutationStats(p *Program) {
	// Pre-conditions
	assert.PathExists(*p.mutantsDIR)

	mutants := listSolidityFiles(*p.mutantsDIR)

	if p.totalGeneratedMutants == 0 {
		assert.True(len(mutants) > 0, "There must be mutants in the pre-analysis phase.")
		assert.True(p.totalUnslainMutants == 0, "Before analysis we don't know if there will be unslain mutants.")

		p.totalGeneratedMutants = int32(len(mutants))
		for _, mutant := range mutants {
			p.perFileGenerated[mutant.Filename]++
		}

		assert.True(len(p.perFileGenerated) > 0, "There should be non-zero keys in the per file generated mutants mapping.")
		assert.True(len(p.perFileUnslain) == 0, "There should be zero keys in the per file unslain mutants mapping.")

		return
	}

	p.totalUnslainMutants = int32(len(mutants))

	for _, mutant := range mutants {
		p.perFileUnslain[mutant.Filename]++
	}

	assert.True(len(p.perFileUnslain) > 0, "There should be non-zero keys in the per file unslain mutants mapping.")
}

func printMutationStats(p *Program) {
	fmt.Printf("--------- Mutation Stats - Start ---------\n\n")

	fmt.Printf("Total mutants generated: %d\n", p.totalGeneratedMutants)
	fmt.Printf("Total mutants unslain: %d\n", p.totalUnslainMutants)
	fmt.Printf("Below is the per file breakdown: \n")
	for file, count := range p.perFileGenerated {
		fmt.Printf("%s: generated %d\n", file, count)
		fmt.Printf("%s: unslain   %d\n", file, p.perFileUnslain[file])
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

			gambitRemappings = append(gambitRemappings, fmt.Sprintf("%s=%s", from, to))
		}
	}

	// Post-conditions
	assert.True(len(gambitRemappings) > 0, "Transformed 0 gambit remappings.")

	return gambitRemappings
}

func testMutations(p *Program) error {
	// Pre-conditions
	assert.True(p.totalGeneratedMutants > 0, "Can't perform analysis if there are no mutants.")
	assert.True(p.totalUnslainMutants == 0, "Before performing the analysis there must be 0 unslain mutants.")

	fmt.Printf("\n\033[32m[Info] Starting the mutation analysis.\033[0m\n\n")

	mutants := listSolidityFiles(*p.mutantsDIR)
	for _, mutant := range mutants {
		// Given: gambit_out/mutants/001/src/staking/Pool.sol
		// We want to copy to /src/staking/Pool.sol

		// From "./src" -> "/src/"
		normalizedContractsDirPath := path.Clean(*p.contractsDIR) + string(os.PathSeparator)

		idx := strings.Index(mutant.PathFromProjectRoot, normalizedContractsDirPath)
		if idx == -1 {
			return fmt.Errorf("[Error] Failed to find the common path between contracts and mutants directories.")
		}

		// From "gambit_out/mutants/001/src/staking/Pool.sol" -> "src/staking/Pool.sol"
		destinationPath := mutant.PathFromProjectRoot[idx:]

		backupPath := destinationPath + ".bak"
		err := copyFile(destinationPath, backupPath)
		if err != nil {
			return fmt.Errorf("[Error] Failed to backup original file %s: %v", destinationPath, err)
		}

		err = copyFile(mutant.PathFromProjectRoot, destinationPath)
		if err != nil {
			return fmt.Errorf(
				"[Error] There was a problem copying the mutant to the destination path: %s, %s",
				destinationPath, err)
		}

		if !testSuitePasses(p, false) {
			fmt.Printf("[Info] Mutant slain 🗡️ (%s)\n", mutant.PathFromProjectRoot)
			mutantFolder := mutant.PathFromProjectRoot[:idx]
			err := os.RemoveAll(mutantFolder)
			if err != nil {
				return fmt.Errorf("Couldn't remove slain mutant from the mutant's dir: %s", err)
			}
		} else {
			fmt.Printf("[Info] Test suite didn't catch the bug. Mutant unslain: %s.\n", mutant.PathFromProjectRoot)
		}

		// Ensure original file is restored when function returns
		err = copyFile(backupPath, destinationPath)
		if err != nil {
			return fmt.Errorf("Couldn't restore the backup file at the destination: %s", err)
		}
		err = os.Remove(backupPath)
		if err != nil {
			return fmt.Errorf("Couldn't remove the backup file: %s", err)
		}
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
