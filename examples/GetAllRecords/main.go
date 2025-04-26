package main

import (
	"fmt"

	"github.com/canhlinh/gozk"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	GetAllScannedEvents(true)
	GetAllScannedEvents(false)
}

func GetAllScannedEvents(tcp bool) {
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

	if events, err := zk.GetAllScannedEvents(); err != nil {
		panic(err)
	} else {
		fmt.Println("Number of events:", len(events))
		for _, event := range events {
			fmt.Println("Event:", event)
		}
	}
}
