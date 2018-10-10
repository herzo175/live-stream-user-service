package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"context"
	"time"

	"github.com/gorilla/mux"

	"github.com/herzo175/live-stream-user-service/src/controllers/admin"
)

func makeServer() *http.Server {
	router := mux.NewRouter()

	healthcheckConfig := admin.HealthcheckConfig{
		Router: router,
	}

	healthcheckConfig.MakeController()

	server := http.Server{
		Addr: "0.0.0.0:3000",
		Handler: router,
	}

	return &server
}

func start() {
	var wait time.Duration
	server := makeServer()
	
	go func() {
		// TODO: get tls cert
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
