package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"backend-challenge/internal/config"
	"backend-challenge/internal/handler"
	"backend-challenge/internal/middleware"
	"backend-challenge/internal/repository"
	"backend-challenge/internal/service"
)

const tokenTTL = 24 * time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}

	repo, err := repository.NewUserRepository(ctx, client.Database(cfg.MongoDatabase))
	if err != nil {
		log.Fatalf("mongo repository: %v", err)
	}

	userService := service.NewUserService(repo, cfg.JWTSecret, tokenTTL)
	authHandler := handler.NewAuthHandler(userService)
	userHandler := handler.NewUserHandler(userService)
	authMiddlewate := middleware.NewAuthMiddlewate(cfg.JWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", authHandler.Register)
	mux.HandleFunc("POST /login", authHandler.Login)
	mux.Handle("GET /users", authMiddlewate(http.HandlerFunc(userHandler.List)))
	mux.Handle("GET /users/{id}", authMiddlewate(http.HandlerFunc(userHandler.Get)))
	mux.Handle("PATCH /users/{id}", authMiddlewate(http.HandlerFunc(userHandler.Update)))
	mux.Handle("DELETE /users/{id}", authMiddlewate(http.HandlerFunc(userHandler.Delete)))

	go logUserCount(userService, 10*time.Second)

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr: addr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      middleware.Logger(mux),
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		log.Printf("server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel = context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}

func logUserCount(s *service.UserService, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		count, err := s.CountUsers(ctx)
		cancel()
		if err != nil {
			log.Printf("count users: %v", err)
			continue
		}
		log.Printf("total users: %d", count)
	}
}
