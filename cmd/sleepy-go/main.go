package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kisukri/sleepy-go/internal/sleepy"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := sleepy.Run(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
