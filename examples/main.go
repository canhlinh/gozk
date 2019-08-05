package main

import (
	"log"

	"github.com/canhlinh/gozk"
)

func main() {
	zkSocket := gozk.NewZkSocket("192.168.0.201", 4370, 0, gozk.DefaultTimezone)
	if err := zkSocket.Connect(); err != nil {
		panic(err)
	}

	c, err := zkSocket.LiveCapture()
	if err != nil {
		panic(err)
	}

	for event := range c {
		log.Println(event)
	}
}
