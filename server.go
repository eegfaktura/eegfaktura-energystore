package main

import (
	"at.ourproject/energystore/graph"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/rest"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/golang/glog"
	"github.com/gorilla/handlers"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"at.ourproject/energystore/config"
)

const defaultPort = "8080"

func captureOsInterrupt() chan bool {
	quit := make(chan bool)
	go func() {
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		for sig := range c {
			glog.V(3).Infof("captured %v, stopping and exiting.", sig)

			quit <- true
			close(quit)

			break
		}
	}()
	return quit
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	var configPath = flag.String("configPath", ".", "Configfile Path")
	flag.Parse()

	glog.V(3).Info("-> Read Config")
	config.ReadConfig(*configPath)
	quit := captureOsInterrupt()

	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := SetupMqttDispatcher(ctx)

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

	//log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(allowedOrigins, allowedHeaders, allowedMethods, allowedCredentials)(r)))

	server := &http.Server{
		Handler: handlers.CORS(allowedOrigins, allowedHeaders, allowedMethods, allowedCredentials)(r),
		Addr:    fmt.Sprintf("0.0.0.0:%s", port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 180 * time.Second,
		ReadTimeout:  180 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen and serve returned err: %v", err)
		}
	}()

	<-quit
	log.Println("got interruption signal")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("server shutdown returned an err: %v\n", err)
	}

	cancel()
	dispatcher.Close()
}

func SetupMqttDispatcher(ctx context.Context) *mqttclient.TopicDispatcher {
	streamer, err := mqttclient.NewMqttStreamer()
	if err != nil {
		panic(err)
	}

	//worker := map[string]mqttclient.Executor{}
	//energyTopicPrefix := viper.GetString("mqtt.energySubscriptionTopic")
	//worker[energyTopicPrefix] = calculation.NewMqttEnergyImporter(ctx)
	//
	//inverterTopicPrefix := viper.GetString("mqtt.inverterSubscriptionTopic")
	//worker[inverterTopicPrefix] = calculation.NewMqttInverterImporter(ctx)

	//dispatcher := mqttclient.NewDispatcher(ctx, streamer, worker)
	//_ = dispatcher
	//
	//if err := streamer.Connect(); err != nil {
	//	panic(err)
	//}
	//
	//for k, _ := range worker {
	//	streamer.SubscribeTopic(ctx, k, nil)
	//}

	energyTopicPrefix := viper.GetString("mqtt.energySubscriptionTopic")
	dispatcher := mqttclient.NewTopicDispatcher(ctx, energyTopicPrefix, streamer)

	if err := streamer.Connect(); err != nil {
		panic(err)
	}

	streamer.SubscribeTopic(ctx, energyTopicPrefix, nil)
	return dispatcher
}
