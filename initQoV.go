package main

import (
	"at.ourproject/energystore/config"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"

	"flag"
	"os"
)

func main() {
	var tenant = flag.String("tenant", "", "tenant to be converted")
	var configPath = flag.String("configPath", ".", "Configfile Path")
	var from = flag.String("from", "", "Start Date")
	var to = flag.String("to", "", "Until Date")
	var initVal = flag.Int("val", 1, "Initial Value")
	flag.Parse()

	println("-> \nRead Config")
	config.ReadConfig(*configPath)

	if tenant == nil || len(*tenant) == 0 {
		os.Exit(1)
	}

	if from == nil || len(*from) == 0 {
		os.Exit(1)
	}

	if to == nil || len(*to) == 0 {
		os.Exit(1)
	}

	dateFrom := *from
	dateTo := *to

	if dateFrom[len(dateFrom)-1] != '/' {
		dateFrom += "/"
	}

	if dateTo[len(dateTo)-1] != '/' {
		dateTo += "/"
	}

	fmt.Printf("From Date String:  %s\n", dateFrom)
	fmt.Printf("Until Date String: %s\n", dateTo)

	db, err := store.OpenStorage(*tenant)
	if err != nil {
		panic(err)
	}
	defer func() { db.Close() }()

	iterCP := db.GetLineRange("CP", dateFrom, dateTo)
	defer iterCP.Close()

	resources := []*model.RawSourceLine{}
	var _lineG1 model.RawSourceLine
	for iterCP.Next(&_lineG1) {
		consumerLen := len(_lineG1.Consumers)

		lineG1 := _lineG1.Copy(consumerLen)
		for i, _ := range lineG1.QoVConsumers {
			lineG1.QoVConsumers[i] = *initVal
		}
		for i, _ := range lineG1.QoVProducers {
			lineG1.QoVProducers[i] = *initVal
		}

		resources = append(resources, &lineG1)
	}

	err = db.SetLines(resources)
	if err != nil {
		fmt.Printf("error write resources %v", err)
	}
}

//func ensureIntSlice(orig []int, size int) []int {
//	l := len(orig)
//	if size >= l {
//		target := make([]int, size+1)
//		copy(target, orig)
//		orig = target
//	}
//	return orig
//}
