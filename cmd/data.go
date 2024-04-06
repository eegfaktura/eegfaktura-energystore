package cmd

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/services"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

var (
	meter string
	begin string
	end   string
	count int16
)

func init() {
	RootCmd.AddCommand(dataCmd)
	dataCmd.Flags().StringVar(&meter, "meter", "",
		"Consider only the keys with specified prefix")
	dataCmd.Flags().StringVar(&begin, "begin", "",
		"Consider only energy data after begin date")
	dataCmd.Flags().StringVar(&end, "end", "",
		"Consider only energy data until end date")
	dataCmd.Flags().Int16Var(&count, "count", 100,
		"Consider only energy data until end date")
}

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Data of persist metering points",
	Long: `
This command prints the energy value of the metring key-value store.
`,
	RunE: handleData,
}

func handleData(cmd *cobra.Command, args []string) error {
	viper.Set("persistence.path", dir)

	beginDate, endDate, err := determinePeriod()
	if err != nil {
		return err
	}

	db, err := store.OpenStorage(tenant, ecId)
	if err != nil {
		return err
	}
	defer db.Close()

	m, err := db.GetMeta("cpmeta/0")
	if err != nil {
		return err
	}

	mapConsumers, mapProducers := mapMeta(m.CounterPoints)
	iter := db.GetLineRange("CP", beginDate, endDate)
	defer iter.Close()

	var cnt int16 = 0
	fmt.Printf("Count %d, cnt %d B: %s E: %s\n", count, cnt, beginDate, endDate)
	fmt.Printf("%22s|%33s|%14s|>Values\n", "Time", "Name", "Direction")
	var _line model.RawSourceLine
	for iter.Next(&_line) {
		consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(&_line)
		preceedCnt := false
		for i := 0; i < consumerMatrix.Rows; i += 1 {
			meta, ok := mapConsumers[i]
			if ok && filter(meter, meta) {
				fmt.Printf("%10s|%33s|%14s|>%4.5f [%s]; %4.5f [%s]; %4.5f [%s]\n", _line.Id, meta.Name, meta.Dir,
					consumerMatrix.GetElm(i, 0), getQoV(_line.QoVConsumers, i, 0, 3),
					consumerMatrix.GetElm(i, 1), getQoV(_line.QoVConsumers, i, 1, 3),
					consumerMatrix.GetElm(i, 2), getQoV(_line.QoVConsumers, i, 2, 3))
				preceedCnt = true
			}
		}
		for i := 0; i < producerMatrix.Rows; i += 1 {
			meta, ok := mapProducers[i]
			if ok && filter(meter, meta) {
				fmt.Printf("%10s|%33s|%14s|>%4.5f; %4.5f\n", _line.Id, meta.Name, meta.Dir, producerMatrix.GetElm(i, 0), producerMatrix.GetElm(i, 1))
				preceedCnt = true
			}
		}
		//fmt.Println()
		if preceedCnt {
			cnt = cnt + 1
			if cnt > count {
				break
			}
		}
	}

	return nil
}

func determinePeriod() (string, string, error) {
	lastEntry, err := services.GetLastEnergyEntry(tenant, ecId)
	if err != nil {
		return "", "", err
	}

	lastEntryDate := utils.StringToTime(lastEntry)
	periodBeginDate := lastEntryDate.Add(time.Hour * 24 * -1)
	if begin != "" {
		if err := checkDateValue(begin); err != nil {
			return "", "", err
		}
		periodBeginDate = utils.StringToTime(begin)
	}

	if end != "" {
		if err := checkDateValue(end); err != nil {
			return "", "", err
		}
		lastEntryDate = utils.StringToTime(end)
		if begin == "" {
			periodBeginDate = lastEntryDate.Add(time.Hour * 24 * -1)
		}
	}

	beginPeriod, err := utils.ConvertUnixTimeToRowId("", periodBeginDate)
	if err != nil {
		return "", "", err
	}
	endPeriod, err := utils.ConvertUnixTimeToRowId("", lastEntryDate)
	if err != nil {
		return "", "", err
	}

	return beginPeriod, endPeriod, nil
}

func printHeadline() {

}

func mapMeta(cps []*model.CounterPointMeta) (cons map[int]*model.CounterPointMeta, prod map[int]*model.CounterPointMeta) {
	cons = map[int]*model.CounterPointMeta{}
	prod = map[int]*model.CounterPointMeta{}

	for _, c := range cps {
		if c.Dir == model.PRODUCER_DIRECTION {
			prod[c.SourceIdx] = c
		} else {
			cons[c.SourceIdx] = c
		}
	}
	return
}

func filter(meter string, meta *model.CounterPointMeta) bool {
	if len(meter) == 0 {
		return true
	}

	if meter == meta.Name {
		return true
	}
	return false
}

func getQoV(arr []int, line, pos, base int) string {
	return fmt.Sprintf("L%d", arr[(line*base)+pos])
}
