package gozk

import (
	"encoding/hex"
	"fmt"
	"time"

	binarypack "github.com/canhlinh/go-binary-pack"
)

// PrintlHex printls bytes to console as HEX encoding
func PrintlHex(title string, buf []byte) {
	fmt.Printf("%s %q\n", title, hex.EncodeToString(buf))
}

func LoadLocation(timezone string) *time.Location {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Local
	}

	return location
}

func newBP() *binarypack.BinaryPack {
	return &binarypack.BinaryPack{}
}

func createCheckSum(p []interface{}) ([]byte, error) {
	l := len(p)
	checksum := 0

	for l > 1 {
		pack, err := newBP().Pack([]string{"B", "B"}, []interface{}{p[0], p[1]})
		if err != nil {
			return nil, err
		}

		unpack, err := newBP().UnPack([]string{"H"}, pack)
		if err != nil {
			return nil, err
		}

		c := unpack[0].(int)
		checksum += c
		p = p[2:]

		if checksum > USHRT_MAX {
			checksum -= USHRT_MAX
		}
		l -= 2
	}

	if l > 0 {
		checksum = checksum + p[len(p)-1].(int)
	}

	for checksum > USHRT_MAX {
		checksum -= USHRT_MAX
	}

	checksum = ^checksum
	for checksum < 0 {
		checksum += USHRT_MAX
	}

	return newBP().Pack([]string{"H"}, []interface{}{checksum})
}

func createHeader(command int, commandString []byte, sessionID int, replyID int) ([]byte, error) {
	buf, err := newBP().Pack([]string{"H", "H", "H", "H"}, []interface{}{command, 0, sessionID, replyID})
	if err != nil {
		return nil, err
	}
	buf = append(buf, commandString...)

	unpackPad := []string{
		"B", "B", "B", "B", "B", "B", "B", "B",
	}

	for i := 0; i < len(commandString); i++ {
		unpackPad = append(unpackPad, "B")
	}

	unpackBuf, err := newBP().UnPack(unpackPad, buf)
	if err != nil {
		return nil, err
	}

	checksumBuf, err := createCheckSum(unpackBuf)
	if err != nil {
		return nil, err
	}

	c, err := newBP().UnPack([]string{"H"}, checksumBuf)
	if err != nil {
		return nil, err
	}
	checksum := c[0].(int)

	replyID++
	if replyID >= USHRT_MAX {
		replyID -= USHRT_MAX
	}

	packData, err := newBP().Pack([]string{"H", "H", "H", "H"}, []interface{}{command, checksum, sessionID, replyID})
	if err != nil {
		return nil, err
	}

	return append(packData, commandString...), nil
}

func createTCPTop(packet []byte) ([]byte, error) {
	top, err := newBP().Pack([]string{"H", "H", "I"}, []interface{}{MACHINE_PREPARE_DATA_1, MACHINE_PREPARE_DATA_2, len(packet)})
	if err != nil {
		return nil, err
	}

	return append(top, packet...), nil
}

func testTCPTop(packet []byte) int {
	if len(packet) <= 8 {
		return 0
	}

	tcpHeader, err := newBP().UnPack([]string{"H", "H", "I"}, packet[:8])
	if err != nil {
		return 0
	}

	if tcpHeader[0].(int) == MACHINE_PREPARE_DATA_1 || tcpHeader[1].(int) == MACHINE_PREPARE_DATA_2 {
		return tcpHeader[2].(int)
	}

	return 0
}

// makeCommKey take a password and session_id and scramble them to send to the time clock.
// copied from commpro.c - MakeKey
func makeCommKey(key, sessionID int, ticks int) ([]byte, error) {
	k := 0

	for i := uint(0); i < 32; i++ {
		if (key & (1 << i)) > 0 {
			k = (k<<1 | 1)
		} else {
			k = k << 1
		}
	}

	k += sessionID

	pack, _ := newBP().Pack([]string{"I"}, []interface{}{k})
	unpack := mustUnpack([]string{"B", "B", "B", "B"}, pack)

	pack, _ = newBP().Pack([]string{"B", "B", "B", "B"}, []interface{}{
		unpack[0].(int) ^ int('Z'),
		unpack[1].(int) ^ int('K'),
		unpack[2].(int) ^ int('S'),
		unpack[3].(int) ^ int('O'),
	})

	unpack = mustUnpack([]string{"H", "H"}, pack)
	pack, _ = newBP().Pack([]string{"H", "H"}, []interface{}{unpack[0], unpack[1]})

	b := 0xff & ticks
	unpack = mustUnpack([]string{"B", "B", "B", "B"}, pack)
	pack, _ = newBP().Pack([]string{"B", "B", "B", "B"}, []interface{}{
		unpack[0].(int) ^ b,
		unpack[1].(int) ^ b,
		b,
		unpack[3].(int) ^ b,
	})

	return pack, nil
}

func mustUnpack(pad []string, data []byte) []interface{} {
	value, err := newBP().UnPack(pad, data)
	if err != nil {
		panic(err)
	}

	return value
}

func getDataSize(rescode int, data []byte) (int, error) {
	if rescode == CMD_PREPARE_DATA {
		sizeUnpack, err := newBP().UnPack([]string{"I"}, data[:4])
		if err != nil {
			return 0, err
		}

		return sizeUnpack[0].(int), nil
	}

	return 0, nil
}
