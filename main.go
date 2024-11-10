package main

import (
	"checkmate/cli"
	"fmt"
	"os"
)

func main() {
	p := cli.New()
	err := cli.Run(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
