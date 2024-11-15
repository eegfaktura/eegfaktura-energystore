package main

import (
	"fmt"

	"github.com/eegfaktura/eegfaktura-energystore/cmd"
)

func main() {
	cmd.Execute()
	fmt.Printf("Program end: %s\n", "now")
}
