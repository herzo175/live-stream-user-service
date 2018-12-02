package main

import (
	"github.com/herzo175/live-stream-user-service/src/util/serverservice"
	"github.com/herzo175/live-stream-user-service/src/util/channels"
	"github.com/herzo175/live-stream-user-service/src/util/emails"
	"github.com/herzo175/live-stream-user-service/src/util/cache"
	"github.com/herzo175/live-stream-user-service/src/util/database"
	"github.com/herzo175/live-stream-user-service/src/bundles/billing"
	"github.com/herzo175/live-stream-user-service/src/bundles/servers"
	"github.com/joho/godotenv"
	"fmt"
	"github.com/herzo175/live-stream-user-service/src/bundles/users"
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

func setupExternalClients() (
	*mux.Router,
	*database.SQLDatabaseClient,
	*emails.EmailClient,
	*channels.Client,
	*cache.RedisCacheClient,
	*serverservice.Service,
) {
	return mux.NewRouter(),
		database.MakeClient(),
		emails.MakeClient(),
		channels.MakeClient(),
		cache.MakeRedisCache(
			os.Getenv("TOKEN_CACHE_ADDRESS"),
			os.Getenv("TOKEN_CACHE_PASSWORD"),
		),
		serverservice.MakeService()
}

func makeServer() *http.Server {
	router, db, emailClient, notificationClient, tokenCache, serverService := setupExternalClients()

	healthcheck.MakeRouter(router)

	userLogic := users.MakeUserLogic(db, emailClient, notificationClient, tokenCache)
	users.MakeRouter(router, userLogic)

	billingLogic := billing.MakeBillingLogicConfig(db, userLogic)
	billing.MakeRouter(router, billingLogic)

	serverLogic := servers.MakeServerLogic(db, serverService, billingLogic, notificationClient)
	servers.MakeRouter(router, serverLogic)

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
