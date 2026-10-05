package main

import (
	"buddyflix/internal/app"
	"log"
	"os"
)

func main() {
	cfg := app.ConfigFromEnv()
	srv, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("BuddyFlix %s listening on %s", app.Version, cfg.ListenAddr)
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
	_ = os.Stdout.Sync()
}
