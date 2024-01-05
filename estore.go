package main

import (
	"at.ourproject/energystore/cmd"
	"fmt"
)

func main() {
	cmd.Execute()
	fmt.Printf("Program end: %s\n", "now")
}
