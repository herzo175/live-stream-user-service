package main

import (
	"github.com/herzo175/live-stream-user-service/src/bundles/billing"
	"github.com/herzo175/live-stream-user-service/src/bundles/servers"
	"github.com/joho/godotenv"
	"fmt"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
	"net"
	"crypto/tls"
	"gopkg.in/mgo.v2"
	"github.com/herzo175/live-stream-user-service/src/bundles"
	"log"
	"net/http"
	"os"
	"os/signal"
	"context"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/handlers"
	// "github.com/mongodb/mongo-go-driver/mongo"

	"github.com/herzo175/live-stream-user-service/src/bundles/admin/healthcheck"
)

func makeServer() *http.Server {
	router := mux.NewRouter()
	// TODO: use go static config library
	tlsConfig := &tls.Config{}

	dialInfo := &mgo.DialInfo{
		Addrs: []string{
			os.Getenv("DB_SHARD_1"),
			os.Getenv("DB_SHARD_2"),
			os.Getenv("DB_SHARD_3"),
		},
		Database: os.Getenv("DB_NAME"),
		Username: os.Getenv("DB_USERNAME"),
		Password: os.Getenv("DB_PASSWORD"),
	}
	dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
		return conn, err
	}

	dbClient, err := mgo.DialWithInfo(dialInfo)

	if err != nil {
		log.Fatal("unable to connect to db: ", err)
	}
	
	controllerConfig := bundles.Controller{}
	controllerConfig.Router = router
	controllerConfig.DB = dbClient.DB(os.Getenv("DB_CLIENT_NAME"))

	healthcheckController := healthcheck.HealthcheckController{Controller: &controllerConfig}
	healthcheckController.MakeRouter()

	userController := users.UserController{Controller: &controllerConfig}
	userController.MakeRouter()

	serverController := servers.ServerController{Controller: &controllerConfig}
	serverController.MakeRouter()

	billingController := billing.BillingController{Controller: &controllerConfig}
	billingController.MakeRouter()

	allowedHeaders := handlers.AllowedHeaders(
		[]string{
			"Accept",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
		},
	)

	allowedOrigins := handlers.AllowedOrigins(
		[]string{"*"},
	)

	allowedMethods := handlers.AllowedMethods(
		[]string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"},
	)

	server := http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%s", os.Getenv("PORT")),
		Handler: handlers.CORS(allowedHeaders, allowedOrigins, allowedMethods)(router),
	}

	return &server
}

func start() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Unable to load environment file")
	}

	var wait time.Duration
	server := makeServer()
	
	go func() {
		// TODO: get tls cert, serve tls
		err := server.ListenAndServe()

		if err != nil {
			log.Println(err)
		}

		log.Println("Server started")
	}()
	
	// graceful shutdown
	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	server.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}

func main() {
	start()
}
