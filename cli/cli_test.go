package cli

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

func FuzzCliFlags(f *testing.F) {
	f.Add("--test-command", "forge test")
	f.Add("--test-command", "forge test --fail-fast")
	f.Add("--mutants-dir", "./gambit_out/mutants")
	f.Add("--skip-gambit", "=true")
	f.Add("--skip-gambit", "=false")
	f.Add("--config-path", "./gambit-config.json")
	f.Add("--config-path", "./gambit-config/json")
	f.Add("--config-path", "./gambit-config.txt")
	f.Add("--config-path", ".//gambit-config.txt")
	f.Add("", "")
	f.Add(" ", " ")

	f.Fuzz(func(t *testing.T, f string, val string) {
		// Fail on panics
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Test panicked with value: %v", r)
			}
		}()
		// Reset flags each run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		// Simulate the os.Args array with flags
		os.Args = []string{"checkmate", f, val}

		// Run the New function which internally calls parseCmdFlags
		program := New() // New will internally call parseCmdFlags

		if program == nil {
			t.Fatal("Program was nil after initialization")
		}

		err := Run(program)
		if err != nil {
			fmt.Println(os.Args)
			fmt.Println(err)
			t.Skip("Run failed with an error that was correctly handled.")
		}
	})
}
