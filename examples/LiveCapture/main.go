package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/canhlinh/gozk"
)

func liveCapture(protocol gozk.Protocol) {
	zk := gozk.NewZK("AWZSOME", protocol, "192.168.100.201", 4370, 0, gozk.DefaultTimezone)
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

	for event := range c {
		if event.Error != nil {
			fmt.Printf("Error: %s\n", event.Error.Error())
			continue
		}
	}
}

func main() {
	// go liveCapture(gozk.TCP)
	go liveCapture(gozk.UDP)

	// Wait system interrupt signal

	// to gracefully shutdown the program

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Shutting down gracefully...")
	close(c)
	os.Exit(0)
}
