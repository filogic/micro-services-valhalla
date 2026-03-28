package main

import (
	"context"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"

	// Blank import triggers init() in function.go
	_ "github.com/filogic/micro-services-valhalla"
)

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	ctx := context.Background()
	if err := funcframework.Start(ctx, port); err != nil {
		log.Fatalf("funcframework.Start: %v", err)
	}
}
