package main

import (
	"fmt"
	"github.com/ChmielewskiKamil/checkmate/cli"
	"os"
)

func main() {
	p := cli.New()
	err := cli.Run(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
