package main

import (
	"fmt"
	"github.com/ChmielewskiKamil/checkmate/cli"
	"os"
)

func main() {
	prog := cli.New()
	if err := cli.Run(prog); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m[Error] %v\033[0m\n", err)
		os.Exit(1)
	}
}
