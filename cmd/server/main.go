package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Gexff/word-search-1v1-go-websocket-server/internal/websocket"
	"github.com/Gexff/word-search-1v1-go-websocket-server/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	gameServer := server.New()
	wsHandler := websocket.New(gameServer)

	http.HandleFunc("/ws", wsHandler.Handle)

	log.Printf("Word Search server listening on :%s\n", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("server failed:", err)
	}
}