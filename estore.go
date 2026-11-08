package main

import (
	"fmt"
	"os"

	"at.ourproject/energystore/cmd"
	"at.ourproject/energystore/internal/logging"
)

func main() {
	// Log lines go to stderr, so the tables on stdout stay clean; an invalid LOG_* setting stops
	// the tool, as it stops the service.
	if err := logging.Configure(os.Environ(), os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cmd.Execute()
	fmt.Printf("Program end: %s\n", "now")
}
