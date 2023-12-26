package gozk

import (
	"errors"
	"fmt"
	"io"
	"time"
)

func (zk *ZK) readSize() (int, error) {
	if _, err := zk.sendCommand(CMD_GET_FREE_SIZES, nil, 1024); err != nil {
		return 0, err
	}

	if len(zk.lastData) >= 80 {
		pad := []string{}
		for i := 0; i < 20; i++ {
			pad = append(pad, "i")
		}
		zk.lastData = zk.lastData[:80]
		return mustUnpack(pad, zk.lastData)[8].(int), nil
	} else if len(zk.lastData) >= 12 {
		fmt.Println("Got length 12")
	}

	return 0, nil
}

func (zk *ZK) readWithBuffer(command, fct, ext int) ([]byte, int, error) {
	commandString, err := newBP().Pack([]string{"b", "h", "i", "i"}, []interface{}{1, command, fct, ext})
	if err != nil {
		return nil, 0, err
	}

	res, err := zk.sendCommand(1503, commandString, 1024)
	if err != nil {
		return nil, 0, err
	}

	if !res.Status {
		return nil, 0, errors.New("RWB Not supported")
	}

	if res.Code == CMD_DATA {

		if need := res.TCPLength - 8 - len(zk.lastData); need > 0 {
			moreData, err := zk.receiveRawData(need)
			if err != nil {
				return nil, 0, err
			}

			data := append(zk.lastData, moreData...)
			return data, len(data), nil
		}

		return zk.lastData, len(zk.lastData), nil
	}

	sizeUnpack, err := newBP().UnPack([]string{"I"}, zk.lastData[1:5])
	if err != nil {
		return nil, 0, err
	}

	size := sizeUnpack[0].(int)
	remain := size % MAX_CHUNK
	packets := (size - remain) / MAX_CHUNK

	data := []byte{}
	start := 0

	for i := 0; i < packets; i++ {

		d, err := zk.readChunk(start, MAX_CHUNK)
		if err != nil {
			return nil, 0, err
		}
		data = append(data, d...)
		start += MAX_CHUNK
	}

	if remain > 0 {
		d, err := zk.readChunk(start, remain)
		if err != nil {
			return nil, 0, err
		}

		data = append(data, d...)
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

func (zk *ZK) readChunk(start, size int) ([]byte, error) {

	for i := 0; i < 3; i++ {
		commandString, err := newBP().Pack([]string{"i", "i"}, []interface{}{start, size})
		if err != nil {
			return nil, err
		}

		res, err := zk.sendCommand(CMD_READ_BUFFER, commandString, size+32)
		if err != nil {
			return nil, err
		}

		data, err := zk.receiveChunk(res.Code, res.TCPLength)
		if err != nil {
			return nil, err
		}

		return data, nil
	}

	return nil, errors.New("can't read chunk")
}

func (zk *ZK) receiveChunk(responseCode int, tcpLength int) ([]byte, error) {

	switch responseCode {
	case CMD_DATA:
		if need := tcpLength - 8 - len(zk.lastData); need > 0 {
			moreData, err := zk.receiveRawData(need)
			if err != nil {
				return nil, err
			}
			return append(zk.lastData, moreData...), nil
		}

		return zk.lastData, nil
	case CMD_PREPARE_DATA:

		data := []byte{}
		size, err := getDataSize(responseCode, zk.lastData)
		if err != nil {
			return nil, err
		}

		dataReceived := []byte{}
		if len(zk.lastData) >= 8+size {
			dataReceived = zk.lastData[8:]
		} else {
			dataReceived = append(zk.lastData[8:], zk.mustReceiveData(size+32)...)
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
		resCode := unpack[0].(int)

		if resCode == CMD_ACK_OK {
			return data, nil
		}

		return []byte{}, nil
	default:
		return nil, errors.New("Invalid reponse")
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
		return nil, data, errors.New("Incorrect tcp packet")
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

		return nil, nil, errors.New("Incorrect response")
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
		return errors.New("Can't verify")
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
		return errors.New("Can't reg event")
	}

	return nil
}

func (zk *ZK) receiveData(size int, timeout time.Duration) ([]byte, error) {
	data := make([]byte, size)

	zk.conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := zk.conn.Read(data)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return nil, errors.New("Failed to received DATA")
	}

	return data[:n], nil
}

func (zk *ZK) ackOK() error {

	buf, err := createHeader(CMD_ACK_OK, nil, zk.sessionID, USHRT_MAX-1)
	if err != nil {
		return err
	}
	top, err := createTCPTop(buf)
	if err != nil {
		return err
	}

	if n, err := zk.conn.Write(top); err != nil {
		return err
	} else if n == 0 {
		return errors.New("Failed to write command")
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
