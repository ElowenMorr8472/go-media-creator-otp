package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	server := &Server{
		sms: &SMSClient{
			BaseURL:    defaultInfraiBaseURL,
			APIKey:     key,
			HTTPClient: &http.Client{},
			MaxRetries: 3,
		},
		assets: NewAssetStore(),
	}
	log.Println("media creator login listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", server.routes()))
}
