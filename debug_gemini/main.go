package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/api/generativelanguage/v1beta"
	"google.golang.org/api/option"
)

func main() {
	godotenv.Load("../.env")
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY is missing")
	}

	ctx := context.Background()
	srv, err := generativelanguage.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Unable to retrieve model service: %v", err)
	}

	res, err := srv.Models.List().Do()
	if err != nil {
		log.Fatalf("Unable to list models: %v", err)
	}

	fmt.Println("Available Models:")
	for _, m := range res.Models {
		fmt.Printf("- %s (Display: %s)\n", m.Name, m.DisplayName)
	}
}
