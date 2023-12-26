package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canhlinh/gozk"
)

func main() {
	zkSocket := gozk.NewZK("192.168.100.201", 4370, 0, gozk.DefaultTimezone)
	if err := zkSocket.Connect(); err != nil {
		panic(err)
	}

	before, err := zkSocket.GetTime()
	if err != nil {
		panic(err)
	}
	new := time.Now().Add(-5 * time.Minute)
	if err := zkSocket.SetTime(new); err != nil {
		panic(err)
	}
	after, err := zkSocket.GetTime()
	if err != nil {
		panic(err)
	}
	zkSocket.Disconnect()
	fmt.Println(before, new.Truncate(time.Second), after)
}

func gracefulQuit(f func()) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan

		log.Println("Stopping...")
		f()

		time.Sleep(time.Second * 1)
		os.Exit(1)
	}()

	for {
		time.Sleep(10 * time.Second) // or runtime.Gosched() or similar per @misterbee
	}
}
