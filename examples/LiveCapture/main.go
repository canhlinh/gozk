package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canhlinh/gozk"
)

func liveCapture(tcp bool, quit chan bool) {
	zk := gozk.NewZK("192.168.100.201", gozk.WithTCP(tcp), gozk.WithTimezone(gozk.DefaultTimezone))
	if err := zk.Connect(); err != nil {
		panic(err)
	}

	defer zk.Disconnect()

	properties, err := zk.GetProperties()
	if err != nil {
		panic(err)
	}
	properties.Println()

	c := make(chan *gozk.ScanEvent)
	if err := zk.StartCapturing(c); err != nil {
		panic(err)
	}

	for {
		select {
		case event := <-c:
			if event == nil {
				fmt.Println("Capture stopped")
				return
			}
			fmt.Printf("Captured event: %+v\n", event)
		case <-quit:
			fmt.Println("Stopping capture...")
			zk.StopCapturing()
			zk.Disconnect()
			return
		}
	}
}

func main() {
	quit := make(chan bool)
	go liveCapture(true, quit)
	go liveCapture(false, quit)

	// Wait system interrupt signal

	// to gracefully shutdown the program

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Shutting down gracefully...")
	close(c)
	close(quit)
	time.Sleep(2 * time.Second) // Allow time for goroutines to finish
	fmt.Println("Shutdown complete.")
	os.Exit(0)
}
