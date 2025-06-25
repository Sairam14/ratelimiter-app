package main

import (
	"log"
	"net/http"

	"ratelimiter-app/internal/handler"
	"ratelimiter-app/pkg/service"
)

func main() {
	svc := service.NewService(service.TokenBucket)
	svc.SetLimit("user123", 20)     // Per-user limit
	svc.SetLimit("apikey-abc", 100) // Per-API-key limit
	// svc.limit = 5 // global default, already set in NewService

	h := handler.NewHandler(svc)
	h.RegisterRoutes()

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
