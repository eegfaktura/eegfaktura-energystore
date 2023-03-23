package main

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/graph"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/rest"
	"context"
	"flag"
	"github.com/99designs/gqlgen/graphql/handler"
	"log"
	"net/http"
	"os"

	"at.ourproject/energystore/config"
	"github.com/spf13/viper"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	var configPath = flag.String("configPath", ".", "Configfile Path")
	flag.Parse()

	println("-> \nRead Config")
	config.ReadConfig(*configPath)

	ctx, _ := context.WithCancel(context.Background())
	SetupMqttDispatcher(ctx)

	r := rest.NewRestServer()
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))
	//r.Handle("/", playground.Handler("GraphQL playground", "/query"))
	r.Handle("/query", srv)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func SetupMqttDispatcher(ctx context.Context) {
	streamer, err := mqttclient.NewMqttStreamer()
	if err != nil {
		panic(err)
	}

	worker := map[string]mqttclient.Executor{}
	energyTopicPrefix := viper.GetString("mqtt.energySubscriptionTopic")
	worker[energyTopicPrefix] = calculation.NewMqttEnergyImporter()

	dispatcher := mqttclient.NewDispatcher(ctx, streamer, worker)
	_ = dispatcher
}
