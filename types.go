package gozk

import (
	"fmt"
	"time"
)

type Response struct {
	Status    bool
	Code      int
	TCPLength int
	Data      []byte
	CommandID int
}

type User struct {
}

type Attendance struct {
	UserID     int64
	AttendedAt time.Time
}

func (r Response) String() string {
	return fmt.Sprintf("Status %v Code %d", r.Status, r.Code)
}
