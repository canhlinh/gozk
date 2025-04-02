package gozk

import (
	"fmt"
	"time"
)

type Response struct {
	Status    bool
	Code      int
	TCPLength int
	CommandID int
	Data      []byte
	ReplyID   int
}

type User struct {
}

type ScanEvent struct {
	DeviceID  string    // An unique identifier for the device
	UserID    int64     // An unique identifier for the user
	Timestamp time.Time // The time when the event was scanned
}

func (r Response) String() string {
	return fmt.Sprintf("Status %v Code %d", r.Status, r.Code)
}
