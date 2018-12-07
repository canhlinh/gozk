package main

import (
	"log"

	"github.com/canhlinh/gozk"
)

func main() {
	zkSocket := gozk.NewZkSocket("192.168.0.202", 4370)
	if err := zkSocket.Connect(); err != nil {
		panic(err)
	}

	attendances, err := zkSocket.GetAttendances()
	if err != nil {
		panic(err)
	}

	for _, attendance := range attendances {
		log.Println(attendance)
	}
}
