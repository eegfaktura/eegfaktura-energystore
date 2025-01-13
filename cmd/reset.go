package cmd

import (
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/store/function"
	"at.ourproject/energystore/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

func init() {
	RootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringVar(&meter, "meter", "",
		"Consider only the keys with specified prefix")
	resetCmd.Flags().StringVar(&begin, "begin", "",
		"Consider only energy data after begin date")
	resetCmd.Flags().StringVar(&end, "end", "",
		"Consider only energy data until end date")
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset data of persist metering points",
	Long: `
This command set data to 0 for the appropriate period of the metring key-value store.
`,
	RunE: handleReset,
}

func handleReset(cmd *cobra.Command, args []string) error {
	viper.Set("persistence.path", dir)

	_begin := utils.StringToTime(begin, time.Now())
	_end := utils.StringToTime(end, time.Now())

	timePeriod, err := function.ToDataTimeRange(_begin, _end)
	if err != nil {
		return err
	}

	db, err := ebow.OpenStorage(tenant, ecId)
	if err != nil {
		return err
	}
	defer db.Close()

	return function.Reset(db, timePeriod, meter)
}
