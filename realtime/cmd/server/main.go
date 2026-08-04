package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/emount4/typing-realtime/internal/auth"
	"github.com/emount4/typing-realtime/internal/config"
	"github.com/emount4/typing-realtime/internal/transport"
)

func main() {
	cfg := config.MustConfig()

	verifier := auth.NewJWTVerifier(cfg.Secret)
	router := transport.SetupRouter(verifier)

	srv := &http.Server{
		Addr:         ":" + cfg.Port, // Гарантируем формат ":8080"
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("shutting down server...")

	// Создаем контекст с таймаутом 5 секунд для корректного завершения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
