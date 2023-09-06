package main

import (
	"at.ourproject/energystore/config"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
	"regexp"

	"flag"
	"os"
)

var (
	dateLine = regexp.MustCompile(`^[0-9]{2}.[0-9]{2}.[0-9]{4}\s[0-9]{2}:[0-9]{2}:[0-9]{2}$`)
)

func main() {
	var tenant = flag.String("tenant", "", "tenant to be converted")
	var configPath = flag.String("configPath", ".", "Configfile Path")
	var meter = flag.String("cp", "", "Meteringpoint")
	var periodStart = flag.String("periodStart", "", "First Period")
	var direction = flag.String("dir", "", "Direction")
	//var to = flag.String("to", "", "Until Date")
	//var initVal = flag.Int("val", 1, "Initial Value")
	flag.Parse()

	println("-> \nRead Config")
	config.ReadConfig(*configPath)

	if tenant == nil || len(*tenant) == 0 {
		os.Exit(1)
	}

	db, err := store.OpenStorage(*tenant)
	if err != nil {
		panic(err)
	}
	defer func() { db.Close() }()

	meta, err := db.GetMeta("cpmeta/0")

	if meter != nil && len(*meter) > 0 {
		var cp *model.CounterPointMeta
		for _, m := range meta.CounterPoints {
			if m.Name == *meter {
				cp = m
				break
			}
		}

		if cp != nil {
			if periodStart != nil && len(*periodStart) > 0 {
				if dateLine.MatchString(*periodStart) {
					cp.PeriodStart = *periodStart
					db.SetMeta(meta)
				} else {
					fmt.Println("Data Format is >DD.MM.YYYY HH:MM:SS>")
				}
			} else if direction != nil && len(*direction) > 0 {
				if *direction == "CONSUMPTION" || *direction == "GENERATION" {
					cp.Dir = model.MeterDirection(*direction)
					db.SetMeta(meta)
				} else {
					fmt.Println("Meter Direction must be either 'CONSUMPTION' or 'GENERATOR'")
				}
			}
		}
		os.Exit(0)
	}

	fmt.Printf("%-4s%-35s%-4s%-15s%-22s%-22s%-10s\n", "Nr.", "Counterpoint", "Idx", "Direction", "Period-Start", "Last Entry", "Count")
	fmt.Println("------------------------------------------------------------------------------------------------------------")
	for i, m := range meta.CounterPoints {
		fmt.Printf("%-4d%-35s%-4d%-15s%-22s%-22s%-10d\n", i, m.Name, m.SourceIdx, m.Dir, m.PeriodStart, m.PeriodEnd, m.Count)
	}
}
