package gozk

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/canhlinh/log4go"
)

const (
	DefaultTimezone = "Asia/Ho_Chi_Minh"
)

var (
	KeepAlivePeriod   = time.Second * 60
	ReadSocketTimeout = 3 * time.Second
)

type ZK struct {
	conn      *net.TCPConn
	sessionID int
	replyID   int
	host      string
	port      int
	pin       int
	loc       *time.Location
	lastData  []byte
	disabled  bool
	capturing chan bool
}

func NewZK(host string, port int, pin int, timezone string) *ZK {
	return &ZK{
		host:      host,
		port:      port,
		pin:       pin,
		loc:       LoadLocation(timezone),
		sessionID: 0,
		replyID:   USHRT_MAX - 1,
	}
}

func (zk *ZK) Connect() error {
	if zk.conn != nil {
		return errors.New("Already connected")
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", zk.host, zk.port), 3*time.Second)
	if err != nil {
		return err
	}

	tcpConnection := conn.(*net.TCPConn)
	if err := tcpConnection.SetKeepAlive(true); err != nil {
		return err
	}

	if err := tcpConnection.SetKeepAlivePeriod(KeepAlivePeriod); err != nil {
		return err
	}

	zk.conn = tcpConnection

	res, err := zk.sendCommand(CMD_CONNECT, nil, 8)
	if err != nil {
		return err
	}

	zk.sessionID = res.CommandID

	if res.Code == CMD_ACK_UNAUTH {
		commandString, _ := makeCommKey(zk.pin, zk.sessionID, 50)
		res, err := zk.sendCommand(CMD_AUTH, commandString, 8)
		if err != nil {
			return err
		}

		if !res.Status {
			return errors.New("unauthorized")
		}
	}

	log.Println("Connected with session_id", zk.sessionID)
	return nil
}

func (zk *ZK) sendCommand(command int, commandString []byte, responseSize int) (*Response, error) {

	if commandString == nil {
		commandString = make([]byte, 0)
	}

	header, err := createHeader(command, commandString, zk.sessionID, zk.replyID)
	if err != nil {
		return nil, err
	}

	top, err := createTCPTop(header)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if n, err := zk.conn.Write(top); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, errors.New("Failed to write command")
	}

	zk.conn.SetReadDeadline(time.Now().Add(ReadSocketTimeout))
	dataReceived := make([]byte, responseSize+8)

	bytesReceived, err := zk.conn.Read(dataReceived)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("GOT ERROR %s ON COMMAND %d", err.Error(), command)
	}

	if bytesReceived == 0 {
		return nil, errors.New("TCP packet invalid")
	}

	receivedHeader, err := newBP().UnPack([]string{"H", "H", "H", "H"}, dataReceived[8:16])
	if err != nil {
		return nil, err
	}

	dataReceived = dataReceived[16:bytesReceived]
	tcpLength := testTCPTop(dataReceived)
	resCode := receivedHeader[0].(int)
	commandID := receivedHeader[2].(int)

	zk.replyID = receivedHeader[3].(int)
	zk.lastData = dataReceived

	switch resCode {
	case CMD_ACK_OK, CMD_PREPARE_DATA, CMD_DATA:
		return &Response{
			Status:    true,
			Code:      resCode,
			TCPLength: tcpLength,
			CommandID: commandID,
		}, nil
	default:
		return &Response{
			Status:    false,
			Code:      resCode,
			TCPLength: tcpLength,
			CommandID: receivedHeader[2].(int),
		}, nil
	}
}

// Disconnect disconnects out of the machine fingerprint
func (zk *ZK) Disconnect() error {
	if zk.conn == nil {
		return errors.New("Already disconnected")
	}

	if _, err := zk.sendCommand(CMD_EXIT, nil, 8); err != nil {
		return err
	}

	if err := zk.conn.Close(); err != nil {
		return err
	}

	zk.conn = nil
	return nil
}

// EnableDevice enables the connected device
func (zk *ZK) EnableDevice() error {

	res, err := zk.sendCommand(CMD_ENABLEDEVICE, nil, 8)
	if err != nil {
		return err
	}

	if !res.Status {
		return errors.New("Failed to enable device")
	}

	zk.disabled = false
	return nil
}

// DisableDevice disable the connected device
func (zk *ZK) DisableDevice() error {
	res, err := zk.sendCommand(CMD_DISABLEDEVICE, nil, 8)
	if err != nil {
		return err
	}

	if !res.Status {
		return errors.New("Failed to disable device")
	}

	zk.disabled = true
	return nil
}

// GetAttendances returns a list of attendances
func (zk *ZK) GetAttendances() ([]*Attendance, error) {
	if err := zk.GetUsers(); err != nil {
		return nil, err
	}

	records, err := zk.readSize()
	if err != nil {
		return nil, err
	}

	data, size, err := zk.readWithBuffer(CMD_ATTLOG_RRQ, 0, 0)
	if err != nil {
		return nil, err
	}

	if size < 4 {
		return []*Attendance{}, nil
	}

	totalSizeByte := data[:4]
	data = data[4:]

	totalSize := mustUnpack([]string{"I"}, totalSizeByte)[0].(int)
	recordSize := totalSize / records
	attendances := []*Attendance{}

	if recordSize == 8 || recordSize == 16 {
		return nil, errors.New("Sorry I don't support this kind of device. I'm lazy")

	}

	for len(data) >= 40 {

		v, err := newBP().UnPack([]string{"H", "24s", "B", "4s", "B", "8s"}, data[:40])
		if err != nil {
			return nil, err
		}

		timestamp, err := zk.decodeTime([]byte(v[3].(string)))
		if err != nil {
			return nil, err
		}

		userID, err := strconv.ParseInt(strings.Replace(v[1].(string), "\x00", "", -1), 10, 64)
		if err != nil {
			return nil, err
		}

		attendances = append(attendances, &Attendance{AttendedAt: timestamp, UserID: userID})
		data = data[40:]
	}

	return attendances, nil
}

// GetUsers returns a list of users
// For now, just run this func. I'll implement this function later on.
func (zk *ZK) GetUsers() error {

	_, err := zk.readSize()
	if err != nil {
		return err
	}

	_, size, err := zk.readWithBuffer(CMD_USERTEMP_RRQ, FCT_USER, 0)
	if err != nil {
		return err
	}

	if size < 4 {
		return nil
	}

	return nil
}

func (zk *ZK) LiveCapture() (chan *Attendance, error) {
	if zk.capturing != nil {
		return nil, errors.New("Is capturing")
	}

	if err := zk.GetUsers(); err != nil {
		return nil, err
	}

	if err := zk.verifyUser(); err != nil {
		return nil, err
	}

	if zk.disabled {
		if err := zk.EnableDevice(); err != nil {
			return nil, err
		}
	}

	if err := zk.regEvent(EF_ATTLOG); err != nil {
		return nil, err
	}

	log4go.Info("Start capturing")
	zk.capturing = make(chan bool, 1)
	c := make(chan *Attendance, 1)

	go func() {

		defer func() {
			log4go.Info("Stopped capturing")
			zk.regEvent(0)
			close(c)
		}()

		for {
			select {
			case <-zk.capturing:
				return
			default:

				data, err := zk.receiveData(1032, KeepAlivePeriod)
				if err != nil && !strings.Contains(err.Error(), "timeout") {
					log4go.Error(err)
					return
				}
				if err := zk.ackOK(); err != nil {
					return
				}

				if len(data) == 0 {
					log4go.Info("Continue")
					continue
				}

				// size := mustUnpack([]string{"H", "H", "I"}, data[:8])[2].(int)
				header := mustUnpack([]string{"H", "H", "H", "H"}, data[8:16])
				data = data[16:]

				if header[0].(int) != CMD_REG_EVENT {
					log.Println("Skip REG EVENT")
					continue
				}

				for len(data) >= 12 {
					unpack := []interface{}{}

					if len(data) == 12 {
						unpack = mustUnpack([]string{"I", "B", "B", "6s"}, data)
						data = data[12:]
					} else if len(data) == 32 {
						unpack = mustUnpack([]string{"24s", "B", "B", "6s"}, data[:32])
						data = data[32:]
					} else if len(data) == 36 {
						unpack = mustUnpack([]string{"24s", "B", "B", "6s", "4s"}, data[:36])
						data = data[36:]
					} else if len(data) >= 52 {
						unpack = mustUnpack([]string{"24s", "B", "B", "6s", "20s"}, data[:52])
						data = data[52:]
					}

					timestamp := zk.decodeTimeHex([]byte(unpack[3].(string)))

					userID, err := strconv.ParseInt(strings.Replace(unpack[0].(string), "\x00", "", -1), 10, 64)
					if err != nil {
						log.Println(err)
						continue
					}

					c <- &Attendance{UserID: userID, AttendedAt: timestamp}
					log.Printf("UserID %v timestampe %v \n", userID, timestamp)
				}
			}
		}

	}()

	return c, nil
}

func (zk ZK) StopCapture() {
	zk.capturing <- false
}
