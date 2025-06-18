package gozk

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

type ZKProperties struct {
	ID           string
	TCP          bool
	Clock        time.Time
	Version      string
	RecordCap    int
	TotalRecords int
	UserCap      int
	TotalUsers   int
	FingerCap    int
	TotalFingers int
}

func (properties ZKProperties) Println() {
	logrus.Println("--------------DEVICE INFORMATION--------------")
	logrus.Println("Device ID:", properties.ID)
	logrus.Println("Device Version:", properties.Version)
	logrus.Println("Device Clock:", properties.Clock.Format(time.RFC3339))
	logrus.Println("Total Users:", properties.TotalUsers)
	logrus.Println("Total Fingers:", properties.TotalFingers)
	logrus.Println("Total Records:", properties.TotalRecords)
	logrus.Println("Finger Capacity:", properties.FingerCap)
	logrus.Println("User Capacity:", properties.UserCap)
	logrus.Println("Record Capacity:", properties.RecordCap)
	if properties.TCP {
		logrus.Println("Protocol: TCP")
	} else {
		logrus.Println("Protocol: UDP")
	}
	logrus.Println("------------------------------------------------")
}

func (zk *ZK) GetProperties() (*ZKProperties, error) {

	version, err := zk.GetFirmwareVersion()
	if err != nil {
		return nil, err
	}

	clock, err := zk.GetTime()
	if err != nil {
		return nil, err
	}

	res, err := zk.sendCommand(CMD_GET_FREE_SIZES, nil, 1024)
	if err != nil {
		return nil, err
	}

	if len(res.Data) >= 80 {
		pad := []string{}
		for i := 0; i < 20; i++ {
			pad = append(pad, "i")
		}
		data, err := unpack(pad, res.Data[:80])
		if err != nil {
			return nil, err
		}
		return &ZKProperties{
			ID:           zk.deviceID,
			TCP:          zk.tcp,
			Version:      version,
			Clock:        clock,
			TotalUsers:   data[4].(int),
			TotalFingers: data[6].(int),
			TotalRecords: data[8].(int),
			FingerCap:    data[14].(int),
			UserCap:      data[15].(int),
			RecordCap:    data[16].(int),
		}, nil
	} else if len(res.Data) >= 12 {
		return nil, errors.New("failed to read data")
	}

	return nil, errors.New("failed to read data")
}

func (zk *ZK) readWithBuffer(command, fct, ext int) ([]byte, int, error) {
	commandString, err := newBP().Pack([]string{"b", "h", "i", "i"}, []interface{}{1, command, fct, ext})
	if err != nil {
		return nil, 0, err
	}

	res, err := zk.sendCommand(CMD_PREPARE_BUFFER, commandString, 1024)
	if err != nil {
		return nil, 0, err
	}

	if !res.Status {
		return nil, 0, errors.New("RWB Not supported")
	}

	if res.Code == CMD_DATA {
		if zk.tcp {
			if need := res.TCPLength - 8 - len(res.Data); need > 0 {
				moreData, err := zk.receiveRawData(need)
				if err != nil {
					return nil, 0, err
				}
				data := append(res.Data, moreData...)
				return data, len(data), nil
			}
			return res.Data, len(res.Data), nil
		}
		return res.Data, len(res.Data), nil
	}

	sizeUnpack, err := newBP().UnPack([]string{"I"}, res.Data[1:5])
	if err != nil {
		return nil, 0, err
	}

	size := sizeUnpack[0].(int)
	remain := size % zk.maxChunk
	packets := (size - remain) / zk.maxChunk

	data := []byte{}
	start := 0
	for i := 0; i < packets; i++ {
		chunk, err := zk.readChunk(start, zk.maxChunk)
		if err != nil {
			return nil, 0, err
		}
		data = append(data, chunk...)
		start += zk.maxChunk
	}

	if remain > 0 {
		chunk, err := zk.readChunk(start, remain)
		if err != nil {
			return nil, 0, err
		}
		data = append(data, chunk...)
		start += remain
	}

	if err := zk.freeData(); err != nil {
		return nil, 0, err
	}

	return data, start, nil
}

func (zk *ZK) freeData() error {
	if _, err := zk.sendCommand(CMD_FREE_DATA, nil, 8); err != nil {
		return err
	}

	return nil
}

func (zk *ZK) receiveRawData(size int) ([]byte, error) {
	data := []byte{}

	for size > 0 {
		chunkData := make([]byte, size)
		n, err := zk.conn.Read(chunkData)
		if err != nil && err != io.EOF {
			return nil, err
		}

		data = append(data, chunkData[:n]...)
		size -= n
	}

	return data, nil
}

func (zk *ZK) tryReadChunk(start, size int) ([]byte, error) {
	commandString, err := newBP().Pack([]string{"i", "i"}, []interface{}{start, size})
	if err != nil {
		return nil, err
	}
	responseSize := UDP_CHUNK_SIZE
	if zk.tcp {
		responseSize = size + 8
	}
	res, err := zk.sendCommand(CMD_READ_BUFFER, commandString, responseSize)
	if err != nil {
		return nil, err
	}

	data, err := zk.receiveChunk(res)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (zk *ZK) readChunk(start, size int) ([]byte, error) {

	for i := 0; i < 3; i++ {
		data, err := zk.tryReadChunk(start, size)
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			return data, nil
		}
	}

	return nil, errors.New("can't read chunk")
}

func (zk *ZK) receiveChunk(res *Response) ([]byte, error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			err := fmt.Errorf("fail to receive chunk: %v\n%s", r, buf[:n])
			logrus.Error(err)
		}
	}()
	switch res.Code {
	case CMD_DATA:
		if zk.tcp {
			if need := res.TCPLength - 8 - len(res.Data); need > 0 {
				moreData, err := zk.receiveRawData(need)
				if err != nil {
					return nil, err
				}
				return append(res.Data, moreData...), nil
			}
			return res.Data, nil
		}
		return res.Data, nil

	case CMD_PREPARE_DATA:
		if zk.tcp {
			data := []byte{}
			size, err := getDataSize(res.Code, res.Data)
			if err != nil {
				return nil, err
			}
			var dataReceived []byte
			if len(res.Data) >= 8+size {
				dataReceived = res.Data[8:]
			} else {
				dataReceived = append(res.Data[8:], zk.mustReceiveData(size+32)...)
			}
			d, brokenHeader, err := zk.receiveTCPData(dataReceived, size)
			if err != nil {
				return nil, err
			}
			data = append(data, d...)

			if len(brokenHeader) < 16 {
				dataReceived = append(brokenHeader, zk.mustReceiveData(16)...)
			} else {
				dataReceived = brokenHeader
			}

			if n := 16 - len(dataReceived); n > 0 {
				dataReceived = append(dataReceived, zk.mustReceiveData(n)...)
			}

			unpack, err := newBP().UnPack([]string{"H", "H", "H", "H"}, dataReceived[8:16])
			if err != nil {
				return nil, err
			}

			resCode := unpack[0].(int)

			if resCode == CMD_ACK_OK {
				return data, nil
			}

			return []byte{}, nil
		}

		data := []byte{}
		for {
			dataReceived := make([]byte, UDP_CHUNK_SIZE)
			n, err := zk.conn.Read(dataReceived)
			if err != nil {
				return nil, err
			}
			dataReceived = dataReceived[:n]
			responseCode := mustUnpack([]string{"H", "H", "H", "H"}, dataReceived[:8])[0].(int)
			switch responseCode {
			case CMD_DATA:
				data = append(data, dataReceived[8:]...)
			case CMD_ACK_OK:
				return data, nil
			default:
				return data, nil
			}
		}
	default:
		return nil, errors.New("invalid response")
	}

}

func (zk *ZK) mustReceiveData(size int) []byte {
	data := make([]byte, size)
	n, err := zk.conn.Read(data)
	if err != nil {
		panic(err)
	}

	if n == 0 {
		panic("Failed to receive data")
	}

	return data[:n]
}

func (zk *ZK) receiveTCPData(packet []byte, size int) ([]byte, []byte, error) {

	tcplength := testTCPTop(packet)
	data := []byte{}

	if tcplength <= 0 {
		return nil, data, errors.New("incorrect tcp packet")
	}

	if n := (tcplength - 8); n < size {

		receivedData, brokenHeader, err := zk.receiveTCPData(packet, n)
		if err != nil {
			return nil, nil, err
		}

		data = append(data, receivedData...)
		size -= len(receivedData)

		packet = append(packet, brokenHeader...)
		packet = append(packet, zk.mustReceiveData(size+16)...)

		receivedData, brokenHeader, err = zk.receiveTCPData(packet, size)
		if err != nil {
			return nil, nil, err
		}
		data = append(data, receivedData...)
		return data, brokenHeader, nil
	}

	packetSize := len(packet)
	responseCode := mustUnpack([]string{"H", "H", "H", "H"}, packet[8:16])[0].(int)

	if packetSize >= size+32 {
		if responseCode == CMD_DATA {
			return packet[16 : size+16], packet[size+16:], nil
		}

		return nil, nil, errors.New("incorrect response")
	}

	if packetSize > size+16 {
		data = append(data, packet[16:size+16]...)
	} else {
		data = append(data, packet[16:packetSize]...)
	}

	size -= (packetSize - 16)
	brokenHeader := []byte{}

	if size < 0 {
		brokenHeader = packet[size:]
	} else if size > 0 {
		rawData, err := zk.receiveRawData(size)
		if err != nil {
			return nil, nil, err
		}
		data = append(data, rawData...)
	}

	return data, brokenHeader, nil
}

func (zk *ZK) decodeTime(packet []byte) (time.Time, error) {
	unpack, err := newBP().UnPack([]string{"I"}, packet)
	if err != nil {
		return time.Time{}, err
	}

	t := unpack[0].(int)

	second := t % 60
	t = t / 60

	minute := t % 60
	t = t / 60

	hour := t % 24
	t = t / 24

	day := t%31 + 1
	t = t / 31

	month := t%12 + 1
	t = t / 12

	year := t + 2000
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, zk.loc), nil
}

func (zk *ZK) verifyUser() error {
	res, err := zk.sendCommand(CMD_STARTVERIFY, nil, 8)
	if err != nil {
		return err
	}

	if !res.Status {
		return errors.New("can't verify")
	}

	return nil
}

func (zk *ZK) regEvent(flag int) error {

	commandString, err := newBP().Pack([]string{"I"}, []interface{}{flag})
	if err != nil {
		return err
	}

	res, err := zk.sendCommand(CMD_REG_EVENT, commandString, 8)
	if err != nil {
		return err
	}

	if !res.Status {
		return errors.New("can't reg event")
	}
	return nil
}

func (zk *ZK) receiveData(size int, timeout time.Duration) ([]byte, error) {
	data := make([]byte, size)
	defer zk.conn.SetReadDeadline(time.Now().Add(ReadSocketTimeout))

	zk.conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := zk.conn.Read(data)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return nil, errors.New("failed to received DATA")
	}

	return data[:n], nil
}

func (zk *ZK) ackOK() error {

	buf, err := createHeader(CMD_ACK_OK, nil, zk.sessionID, USHRT_MAX-1)
	if err != nil {
		return err
	}

	if zk.tcp {
		top, err := createTCPTop(buf)
		if err != nil {
			return err
		}
		if _, err := zk.conn.Write(top); err != nil {
			return err
		}
	} else if _, err := zk.conn.Write(buf); err != nil {
		return err
	}
	return nil
}

func (zk *ZK) decodeTimeHex(timehex []byte) time.Time {
	data := mustUnpack([]string{"B", "B", "B", "B", "B", "B"}, timehex)
	year := data[0].(int)
	month := data[1].(int)
	day := data[2].(int)
	hour := data[3].(int)
	minute := data[4].(int)
	second := data[5].(int)

	year += 2000
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, zk.loc)
}

func (zk *ZK) encodeTime(t time.Time) int {
	return (((t.Year()%100)*12*31+((int(t.Month())-1)*31)+t.Day()-1)*
		(24*60*60) + (t.Hour()*60+t.Minute())*60 + t.Second())
}

func (zk *ZK) sendCommand(command int, commandString []byte, responseSize int) (*Response, error) {
	if zk.capturing != nil {
		return nil, errors.New("cannot send command when capturing")
	}

	if commandString == nil {
		commandString = make([]byte, 0)
	}

	header, err := createHeader(command, commandString, zk.sessionID, zk.replyID)
	if err != nil {
		return nil, err
	}

	var receivedHeader []interface{}
	var tcpLength int
	var data []byte

	if zk.tcp {
		top, err := createTCPTop(header)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n, err := zk.conn.Write(top); err != nil {
			return nil, err
		} else if n == 0 {
			return nil, errors.New("failed to write command")
		}
		zk.conn.SetReadDeadline(time.Now().Add(ReadSocketTimeout))
		tcpDataRecieved := make([]byte, responseSize+8)
		bytesReceived, err := zk.conn.Read(tcpDataRecieved)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("GOT ERROR %s ON COMMAND %d", err.Error(), command)
		}
		tcpLength = testTCPTop(tcpDataRecieved)
		if bytesReceived == 0 || tcpLength == 0 {
			return nil, errors.New("TCP packet invalid")
		}
		receivedHeader, err = newBP().UnPack([]string{"H", "H", "H", "H"}, tcpDataRecieved[8:16])
		if err != nil {
			return nil, err
		}
		data = tcpDataRecieved[16:bytesReceived]
	} else {
		if n, err := zk.conn.Write(header); err != nil {
			return nil, err
		} else if n == 0 {
			return nil, errors.New("failed to write command")
		}
		zk.conn.SetReadDeadline(time.Now().Add(ReadSocketTimeout))
		udpDataRecieved := make([]byte, responseSize)
		n, err := zk.conn.Read(udpDataRecieved)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("GOT ERROR %s ON COMMAND %d", err.Error(), command)
		}
		udpDataRecieved = udpDataRecieved[:n]
		receivedHeader, err = newBP().UnPack([]string{"H", "H", "H", "H"}, udpDataRecieved[:8])
		if err != nil {
			return nil, err
		}
		data = udpDataRecieved[8:]
	}

	resCode := receivedHeader[0].(int)
	commandID := receivedHeader[2].(int)
	zk.replyID = receivedHeader[3].(int)

	switch resCode {
	case CMD_ACK_OK, CMD_PREPARE_DATA, CMD_DATA:
		return &Response{
			Status:    true,
			Code:      resCode,
			TCPLength: tcpLength,
			CommandID: commandID,
			Data:      data,
			ReplyID:   zk.replyID,
		}, nil
	default:
		return &Response{
			Status:    false,
			Code:      resCode,
			TCPLength: tcpLength,
			CommandID: commandID,
			Data:      data,
			ReplyID:   zk.replyID,
		}, nil
	}
}
