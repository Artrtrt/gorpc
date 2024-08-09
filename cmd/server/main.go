package main

import (
	"context"
	"fmt"
	"gorpc/internal/app"
	"gorpc/internal/components/server/config"
	"gorpc/internal/utils"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	conf, err := config.NewConfig("./internal/configs/server_config.json")
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := utils.PemToPrivateKey("./private.pem")
	if err != nil {
		log.Fatal(err)
		return
	}

	server := app.NewServer(conf, privateKey)

	log.Println("booting server...")
	err = server.Boot()
	if err != nil {
		log.Println(fmt.Errorf("error in server boot %w", err))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("starting server...")
		err = server.Start(ctx)
		if err != nil {
			log.Println(fmt.Errorf("error in server run %w", err))
		}

		stop()
	}()

	go func() {
		<-ctx.Done()
		log.Println("stopping server...")
		err = server.Stop()
		if err != nil {
			log.Println(fmt.Errorf("error while stopping server: %w", err))
		}
		log.Println("server stopped")
	}()

	wg.Wait()
	os.Exit(1)
}
