package main

import (
	"github.com/canhlinh/gozk"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	// GetAllScannedEvents(gozk.TCP)
	GetAllScannedEvents(gozk.UDP)
}

func GetAllScannedEvents(protocol gozk.Protocol) {
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

	if _, err := zk.GetAllScannedEvents(); err != nil {
		panic(err)
	} else {
		// for _, event := range events {
		// 	fmt.Printf("Event: %s\n", event)
		// }
	}
}
