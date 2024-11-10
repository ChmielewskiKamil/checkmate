package cli

import (
	"checkmate/assert"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		"./gambit-config.json",
		"Specify the path to the gambit config json file. Default is './gambit-config.json'.",
	)

	flag.Parse()

	p.testCMD = testCMD
	p.mutantsDIR = mutantsDIR
	p.skipGambit = skipGambit
	p.gambitConfigPath = gambitConfigPath

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
	if _, err := os.Stat(*p.gambitConfigPath); os.IsNotExist(err) {
		fmt.Printf("[Info] Gambit config json file does not exist.\n")
		return false
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "[Error] There was a problem accessing the path: %s.\n", *p.gambitConfigPath)
		return false
	}

	fmt.Printf("[Info] Gambit config found at: %s.\n", *p.gambitConfigPath)
	return true
}

func generateGambitConfig(p *Program) error {
	// Pre-conditions
	assert.PathNotExists(*p.gambitConfigPath)

	// Actions
	// TODO: Implement config generation
	return fmt.Errorf("\033[31m[Error] Gambit config generation is not yet implemented.\n        Please provide your own config file. Thanks!\033[0m")

	// Post-conditions
	// assert.PathExists(*gambitConfigPath)
	// assert.NotEmpty(*gambitConfigPath)
	// return nil
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

func testMutations(p *Program) {}
