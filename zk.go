package gozk

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	DefaultTimezone = "Asia/Ho_Chi_Minh"
)

var (
	KeepAlivePeriod   = time.Minute
	ReadSocketTimeout = 3 * time.Second
)

type ZK struct {
	conn      net.Conn
	tcp       bool
	sessionID int
	replyID   int
	host      string
	port      int
	pin       int
	loc       *time.Location
	disabled  bool
	capturing chan bool
	deviceID  string
	maxChunk  int
}

func NewZK(host string, opts ...Option) *ZK {
	option := composeOption(opts...)
	return &ZK{
		sessionID: 0,
		replyID:   USHRT_MAX - 1,
		host:      host,
		port:      option.port,
		pin:       option.pin,
		loc:       option.timezone,
		deviceID:  option.deviceID,
		tcp:       option.useTCP,
		maxChunk:  option.maxChunk,
	}
}

func (zk *ZK) Connect() error {
	if zk.conn != nil {
		return errors.New("already connected")
	}
	conn, err := newSocketConnection(zk.tcp, zk.host, zk.port)
	if err != nil {
		return err
	}
	zk.conn = conn
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

	logrus.Info("Connected to the device with session_id:", zk.sessionID)
	return nil
}

// Disconnect disconnects out of the machine fingerprint
func (zk *ZK) Disconnect() error {
	if zk.conn == nil {
		return errors.New("already disconnected")
	}
	defer logrus.Info("Device has been disconnected")

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
		return errors.New("failed to enable device")
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
		return errors.New("failed to disable device")
	}

	zk.disabled = true
	return nil
}

// GetAllScannedEvents returns total attendances from the connected device
func (zk *ZK) GetAllScannedEvents() ([]*ScanEvent, error) {
	properties, err := zk.GetProperties()
	if err != nil {
		return nil, err
	}

	data, size, err := zk.readWithBuffer(CMD_ATTLOG_RRQ, 0, 0)
	if err != nil {
		return nil, err
	}
	if size < 4 {
		return []*ScanEvent{}, nil
	}

	totalSizeByte := data[:4]
	data = data[4:]

	totalSize := mustUnpack([]string{"I"}, totalSizeByte)[0].(int)
	recordSize := totalSize / properties.TotalRecords

	attendances := []*ScanEvent{}
	if recordSize == 8 || recordSize == 16 {
		return nil, errors.New("sorry but I'm too lazy to implement this")
	}

	for len(data) >= 40 {

		v, err := newBP().UnPack([]string{"H", "24s", "B", "4s", "B", "8s"}, ljust(data, 40))
		if err != nil {
			return nil, err
		}

		timestamp, err := zk.decodeTime([]byte(v[3].(string)))
		if err != nil {
			return nil, err
		}

		userID, err := strconv.ParseInt(strings.ReplaceAll(v[1].(string), "\x00", ""), 10, 64)
		if err != nil {
			return nil, err
		}
		attendances = append(attendances, &ScanEvent{DeviceID: zk.deviceID, Timestamp: timestamp, UserID: userID})
		data = data[40:]
	}

	return attendances, nil
}

// GetUsers returns a list of users
// For now, just run this func. I'll implement this function later on.
func (zk *ZK) GetUsers() error {

	_, size, err := zk.readWithBuffer(CMD_USERTEMP_RRQ, FCT_USER, 0)
	if err != nil {
		return err
	}

	if size < 4 {
		return nil
	}

	return nil
}

func (zk *ZK) StartCapturing(outerChan chan<- *ScanEvent) error {
	if zk.capturing != nil {
		return errors.New("already capturing")
	}

	if zk.disabled {
		return errors.New("device is disabled")
	}

	if err := zk.verifyUser(); err != nil {
		return err
	}

	if err := zk.regEvent(EF_ATTLOG); err != nil {
		return err
	}

	logrus.Info("Start capturing device_id:", zk.deviceID)
	zk.capturing = make(chan bool, 1)

	onConnectionError := func(err error) {
		outerChan <- &ScanEvent{DeviceID: zk.deviceID, Error: err}
	}

	go func() {
		defer func() {
			logrus.Info("Stopped capturing")
			zk.regEvent(0)
		}()

		for {
			select {
			case <-zk.capturing:
				return
			default:
				data, err := zk.receiveData(1032, KeepAlivePeriod)
				if err != nil {
					if !strings.Contains(err.Error(), "timeout") {
						onConnectionError(err)
						return
					}
				}

				if err := zk.ackOK(); err != nil {
					onConnectionError(err)
					return
				}

				if len(data) == 0 {
					continue
				}

				var header []interface{}
				if zk.tcp {
					header = mustUnpack([]string{"H", "H", "H", "H"}, data[8:16])
					data = data[16:]
				} else {
					header = mustUnpack([]string{"H", "H", "H", "H"}, data[:8])
					data = data[8:]
				}

				if header[0].(int) != CMD_REG_EVENT {
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
						onConnectionError(err)
						return
					}
					event := &ScanEvent{DeviceID: zk.deviceID, UserID: userID, Timestamp: timestamp}
					outerChan <- event
					logrus.Println("ScanEvent", event.String())
				}
			}
		}

	}()

	return nil
}

func (zk *ZK) StopCapturing() {
	zk.capturing <- true
	close(zk.capturing)

}

func (zk ZK) Clone() *ZK {
	return &ZK{
		host:      zk.host,
		port:      zk.port,
		pin:       zk.pin,
		loc:       zk.loc,
		sessionID: 0,
		replyID:   USHRT_MAX - 1,
		tcp:       zk.tcp,
		capturing: nil,
		deviceID:  zk.deviceID,
		maxChunk:  zk.maxChunk,
	}
}

func (zk *ZK) GetFirmwareVersion() (string, error) {
	res, err := zk.sendCommand(CMD_GET_VERSION, nil, 1024)
	if err != nil {
		return "", err
	}
	if !res.Status {
		return "", errors.New("can not get version")
	}
	return string(res.Data), nil

}

func (zk *ZK) GetTime() (time.Time, error) {
	res, err := zk.sendCommand(CMD_GET_TIME, nil, 1032)
	if err != nil {
		return time.Now(), err
	}
	if !res.Status {
		return time.Now(), errors.New("can not get time")
	}

	return zk.decodeTime(res.Data[:4])
}

func (zk *ZK) SetTime(t time.Time) error {
	truncatedTime := t.Truncate(time.Second)
	logrus.Info("Set new time:", truncatedTime)

	commandString, err := newBP().Pack([]string{"I"}, []interface{}{zk.encodeTime(truncatedTime)})
	if err != nil {
		return err
	}
	res, err := zk.sendCommand(CMD_SET_TIME, commandString, 8)
	if err != nil {
		return err
	}
	if !res.Status {
		return errors.New("can not set time")
	}
	return nil
}

func (zk *ZK) UnlockTheDoor(delayInSeconds int) error {
	commandString := mustPack([]string{"I"}, []interface{}{delayInSeconds * 10})
	res, err := zk.sendCommand(CMD_UNLOCK, commandString, 8)
	if err != nil {
		return err
	}
	if !res.Status {
		return errors.New("can not unlock the door")
	}
	return nil
}

func (zk *ZK) WriteLCD(text string) error {
	if len(text) > 32 {
		text = text[:32]
	}
	commandString := mustPack([]string{"H", "B"}, []interface{}{0, 0})
	commandString = append(commandString, []byte(" "+text)...)
	res, err := zk.sendCommand(CMD_WRITE_LCD, commandString, 8)
	if err != nil {
		return err
	}
	if !res.Status {
		return errors.New("can not write to LCD")
	}
	return nil
}
