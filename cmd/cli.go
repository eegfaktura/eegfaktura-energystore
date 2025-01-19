package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

var tenant, dir, ecId string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:               "eStore",
	Short:             "Tools to manage Energy Store database.",
	PersistentPreRunE: validateRootCmdArgs,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&tenant, "tenant", "",
		"Tenant which is supposed to inspect. (required)")

	RootCmd.PersistentFlags().StringVar(&dir, "dir", "",
		"Directory where the value log files are located.")

	RootCmd.PersistentFlags().StringVar(&ecId, "ecId", "",
		"CommunityId of.")
}

func validateRootCmdArgs(cmd *cobra.Command, args []string) error {
	if strings.HasPrefix(cmd.Use, "help ") { // No need to validate if it is help
		return nil
	}
	if tenant == "" {
		return errors.New("--tenant not specified")
	}
	if dir == "" {
		return errors.New("--dir not specified")
	}
	if ecId == "" {
		return errors.New("--ecId not specified")
	}
	return nil
}
