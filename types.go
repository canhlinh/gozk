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
	Error     error     // An error if the event is invalid
}

func (event ScanEvent) String() string {
	return fmt.Sprintf("device_id:%s user_id:%d at:%v", event.DeviceID, event.UserID, event.Timestamp.Format(time.RFC3339))
}

func (r Response) String() string {
	return fmt.Sprintf("Status %v Code %d", r.Status, r.Code)
}
