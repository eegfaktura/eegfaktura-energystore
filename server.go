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
	"github.com/gorilla/handlers"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"

	"at.ourproject/energystore/config"
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

	ctx, cancel := context.WithCancel(context.Background())
	SetupMqttDispatcher(ctx)
	defer cancel()

	r := rest.NewRestServer()
	//r.Use(middleware.GQLMiddleware(viper.GetString("jwt.pubKeyFile")))
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))
	//r.Handle("/", playground.Handler("GraphQL playground", "/query"))
	r.Handle("/query", srv)

	allowedOrigins := handlers.AllowedOrigins([]string{"*"})
	allowedHeaders := handlers.AllowedHeaders(
		[]string{"X-Requested-With",
			"Accept",
			"Accept-Encoding",
			"Accept-Language",
			"Host",
			"authorization",
			"Content-Type",
			"Content-Length",
			"X-Content-Type-Options",
			"Origin",
			"Connection",
			"Referer",
			"User-Agent",
			"Sec-Fetch-Dest",
			"Sec-Fetch-Mode",
			"Sec-Fetch-Site",
			"Cache-Control",
			"tenant"})
	allowedMethods := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})
	allowedCredentials := handlers.AllowCredentials()

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)

	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(allowedOrigins, allowedHeaders, allowedMethods, allowedCredentials)(r)))
}

func SetupMqttDispatcher(ctx context.Context) {
	streamer, err := mqttclient.NewMqttStreamer()
	if err != nil {
		panic(err)
	}

	worker := map[string]mqttclient.Executor{}
	energyTopicPrefix := viper.GetString("mqtt.energySubscriptionTopic")
	worker[energyTopicPrefix] = calculation.NewMqttEnergyImporter(ctx)

	inverterTopicPrefix := viper.GetString("mqtt.inverterSubscriptionTopic")
	worker[inverterTopicPrefix] = calculation.NewMqttInverterImporter(ctx)

	dispatcher := mqttclient.NewDispatcher(ctx, streamer, worker)
	_ = dispatcher
}
