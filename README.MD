# GOZK

Get inspired from [this project](https://github.com/fananimi/pyzk)

GOZK is a library to help gopher interact with ZK fingerprint machine

## Features:

- Get all check-in list in a ZK fingerprint machine
- Get all users
- Reatime capturing events

## Getting started

```
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

```
