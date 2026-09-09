package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"at.ourproject/energystore/graph"
	"at.ourproject/energystore/graph/generated"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/rest"
	"at.ourproject/energystore/store/ebow"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/golang/glog"
	"github.com/gorilla/handlers"
	"github.com/spf13/viper"

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
	r.Handle("/query", middleware.GQLProtect(srv))

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
			"tenant",
			"X-tenant"})
	allowedMethods := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})
	allowedCredentials := handlers.AllowCredentials()

	glog.Infof("connect to http://localhost:%s/ for GraphQL playground", port)

	//log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(allowedOrigins, allowedHeaders, allowedMethods, allowedCredentials)(r)))

	server := &http.Server{
		Handler: handlers.CORS(allowedOrigins, allowedHeaders, allowedMethods, allowedCredentials)(r),
		Addr:    fmt.Sprintf("0.0.0.0:%s", port),
		// Good practice: enforce timeouts for servers you create!
		//
		// WriteTimeout deckt in Go den GESAMTEN Handler plus das Schreiben der
		// Antwort ab. Der Energiedatenexport erzeugt die komplette XLSX, bevor
		// das erste Byte fliesst -- die Erzeugungsdauer zaehlt also voll gegen
		// diese Frist. Bei 180s brach der Export einer EEG mit 644 Mitgliedern
		// nach ~218s mit "write tcp ...: i/o timeout" ab, obwohl die Datei
		// fertig war. Es gibt EEGs mit ueber 1000 Mitgliedern.
		//
		// Bewusst HOEHER als das Ingress-Limit (600s): so schneidet im Ernstfall
		// der Proxy sauber ab, statt dass die Anwendung mitten im Schreiben in
		// ihre eigene Frist laeuft. Der Ingress bleibt die wirksame Grenze.
		//
		// Das ist eine Reserve, keine Loesung: ein Jahresexport oder weiteres
		// Wachstum sprengt auch 900s. Die strukturelle Antwort ist die
		// Entkopplung des Exports (konzept-async-energy-export.md).
		WriteTimeout: 900 * time.Second,
		// ReadTimeout bleibt bei 180s -- gelesen wird nur die Anforderung
		// (Teilnehmerliste als JSON), das ist auch bei 1000+ Mitgliedern schnell.
		ReadTimeout: 180 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			glog.Fatalf("listen and serve returned err: %v", err)
		}
	}()

	<-quit
	glog.Info("got interruption signal")
	if err := server.Shutdown(context.Background()); err != nil {
		glog.Infof("server shutdown returned an err: %v", err)
	}

	cancel()
	dispatcher.Close()
	ebow.ClosePool()
}

func SetupMqttDispatcher(ctx context.Context) *mqttclient.TopicDispatcher {
	streamer, err := mqttclient.NewMqttStreamer()
	if err != nil {
		panic(err)
	}

	energyTopicPrefix := viper.GetString("mqtt.energySubscriptionTopic")
	dispatcher := mqttclient.NewTopicDispatcher(ctx, energyTopicPrefix, streamer)

	if err := streamer.Connect(); err != nil {
		panic(err)
	}

	streamer.SubscribeTopic(ctx, energyTopicPrefix, nil)
	return dispatcher
}
