package gozk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TODO: Adds more test to the lib
// I'm too lazy to work to this project

const (
	testZkHost = "192.168.0.202"
	testZkPort = 4370
)

func TestSocketcreateHeader(t *testing.T) {
	socket := &ZkSocket{}
	_, err := socket.createHeader(CMD_CONNECT, nil, 0, USHRT_MAX-1)
	require.NoError(t, err)
}

func TestSocketConnect(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort)
	err := socket.Connect(0)
	require.NoError(t, err)
	defer socket.Disconnect()
}

func TestSocketGetAttendances(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort)
	err := socket.Connect(0)
	require.NoError(t, err)
	defer socket.Disconnect()

	_, err = socket.GetAttendances()
	require.NoError(t, err)
}

func TestSocketGetUsers(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort)
	require.NoError(t, socket.Connect(0))

	_, err := socket.GetUsers()
	require.NoError(t, err)
}
