package main

import (
	"fmt"
	"log"

	"github.com/Adi-ty/go-minio/internal/storage"
	transportHttp "github.com/Adi-ty/go-minio/internal/transport/http"
)

func Run() error {
	fmt.Println("Starting up our application")

	storageService := storage.NewService()

	httpHandler := transportHttp.NewHandler(storageService)
	if err := httpHandler.Serve(); err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("minIO Storage API")
	if err := Run(); err != nil {
		log.Fatalf("Error starting the application: %v", err)
	}
}
