package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canhlinh/gozk"
)

func main() {
	zkSocket := gozk.NewZkSocket("192.168.0.201", 4370, 0, gozk.DefaultTimezone)
	if err := zkSocket.Connect(); err != nil {
		panic(err)
	}

	quit := make(chan bool)
	c, err := zkSocket.LiveCapture(quit)
	if err != nil {
		panic(err)
	}

	go func() {
		for event := range c {
			log.Println(event)
		}
	}()

	f := func() {
		quit <- true
		zkSocket.Disconnect()
	}

	gracefulQuit(f)
}

func gracefulQuit(f func()) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan

		log.Println("Stopping...")
		f()
		os.Exit(1)
	}()

	for {
		time.Sleep(10 * time.Second) // or runtime.Gosched() or similar per @misterbee
	}
}
