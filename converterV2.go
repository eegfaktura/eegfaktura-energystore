package main

import (
	"at.ourproject/energystore/config"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"

	"flag"
	"os"
	"strings"
)

func convertRowId(prefix, id string) string {
	rowIdTime, err := utils.ConvertRowIdToTime(prefix, id)
	if err != nil {
		panic(err)
	}
	tRowId, err := utils.ConvertUnixTimeToRowId("CP/", rowIdTime)
	if err != nil {
		panic(err)
	}

	return tRowId
}

func main() {

	var tenant = flag.String("tenant", "", "tenant to be converted")
	var ecId = flag.String("ecId", "", "communityId of eeg")
	var configPath = flag.String("configPath", ".", "Configfile Path")
	flag.Parse()

	println("-> \nRead Config")
	config.ReadConfig(*configPath)

	if tenant == nil || len(*tenant) == 0 {
		os.Exit(1)
	}

	oldTenant := strings.ToUpper(*tenant)
	newTenant := oldTenant + "_new"

	dbOld, err := store.OpenStorage(oldTenant, *ecId)
	if err != nil {
		panic(err)
	}
	defer func() { dbOld.Close() }()

	dbNew, err := store.OpenStorage(newTenant, *ecId)
	if err != nil {
		panic(err)
	}
	defer func() { dbNew.Close() }()

	sourceLine := model.RawSourceLine{}
	targets := map[string]*model.RawSourceLine{}

	itOld := dbOld.GetLinePrefix("CP-G.01/")
	counter := 0
	for itOld.Next(&sourceLine) {
		tConsumer := model.NewMatrix(len(sourceLine.Consumers), 3)
		tProducer := model.NewMatrix(len(sourceLine.Producers), 2)

		for i, l := range sourceLine.Consumers {
			tConsumer.SetElm(i, 0, l)
		}

		for i, l := range sourceLine.Producers {
			tProducer.SetElm(i, 0, l)
		}

		tRowId := convertRowId("CP-G.01", sourceLine.Id)
		t := model.RawSourceLine{Id: tRowId, Consumers: tConsumer.Elements, Producers: tProducer.Elements}
		targets[tRowId] = &t
		//counter += 1
		//if counter > 10000 {
		//	if err = dbNew.SetLines(targets); err != nil {
		//		panic(err)
		//	}
		//	targets = []*model.RawSourceLine{}
		//	counter = 0
		//}
	}
	itOld.Close()

	itOld = dbOld.GetLinePrefix("CP-G.02/")
	counter = 0
	for itOld.Next(&sourceLine) {
		tRowId := convertRowId("CP-G.02", sourceLine.Id)
		t, ok := targets[tRowId]
		if !ok {
			continue
		}

		tConsumer := model.MakeMatrix(t.Consumers, len(t.Consumers)/3, 3)
		tProducer := model.MakeMatrix(t.Producers, len(t.Producers)/2, 2)

		for i, l := range sourceLine.Consumers {
			tConsumer.SetElm(i, 1, l)
		}

		for i, l := range sourceLine.Producers {
			tProducer.SetElm(i, 1, l)
		}
		targets[tRowId] = &model.RawSourceLine{Id: tRowId, Consumers: tConsumer.Elements, Producers: tProducer.Elements}
	}
	itOld.Close()

	itOld = dbOld.GetLinePrefix("CP-G.03/")
	counter = 0
	for itOld.Next(&sourceLine) {
		tRowId := convertRowId("CP-G.03", sourceLine.Id)
		t, ok := targets[tRowId]
		if !ok {
			continue
		}

		tConsumer := model.MakeMatrix(t.Consumers, len(t.Consumers)/3, 3)

		for i, l := range sourceLine.Consumers {
			tConsumer.SetElm(i, 2, l)
		}
		targets[tRowId] = &model.RawSourceLine{Id: tRowId, Consumers: tConsumer.Elements, Producers: t.Producers}
	}
	itOld.Close()

	counter = 0
	lines := []*model.RawSourceLine{}
	for _, v := range targets {
		lines = append(lines, v)
		counter += 1
		if counter > 10000 {
			if err = dbNew.SetLines(lines); err != nil {
				panic(err)
			}
			lines = []*model.RawSourceLine{}
			counter = 0
		}
	}

	if err = dbNew.SetLines(lines); err != nil {
		panic(err)
	}

	meta, err := dbOld.GetMeta("cpmeta/0")
	if err != nil {
		panic(err)
	}
	dbNew.SetMeta(meta)
}
