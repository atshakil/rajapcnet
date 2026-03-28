package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"nvr/internal/api"
	"nvr/internal/config"
	"nvr/internal/database"
	"nvr/internal/motion"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	motionMgr := motion.NewManager(db)
	go motionMgr.Start(ctx)

	srv := api.NewServer(db, cfg, motionMgr)

	go func() {
		log.Printf("nvr listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	log.Println("shutting down")
}
