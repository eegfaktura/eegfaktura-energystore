package main

import (
	"fmt"

	"at.ourproject/energystore/cmd"
)

func main() {
	cmd.Execute()
	fmt.Printf("Program end: %s\n", "now")
}
