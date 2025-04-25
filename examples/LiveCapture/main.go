package main

import (
	"fmt"

	"github.com/canhlinh/gozk"
)

func main() {
	zk := gozk.NewZK("AWZSOME", gozk.TCP, "192.168.100.201", 4370, 0, gozk.DefaultTimezone)
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
