package gozk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TODO: Adds more test to the lib
// I'm too lazy to work to this project

const (
	testZkHost   = "192.168.0.201"
	testZkPort   = 4370
	testTimezone = "Asia/Ho_Chi_Minh"
)

func TestSocketcreateHeader(t *testing.T) {
	socket := &ZkSocket{}
	_, err := socket.createHeader(CMD_CONNECT, nil, 0, USHRT_MAX-1)
	require.NoError(t, err)
}

func TestSocketConnect(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort, 0, testTimezone)
	err := socket.Connect()
	require.NoError(t, err)
	defer socket.Destroy()
}

func TestSocketGetAttendances(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort, 0, testTimezone)
	err := socket.Connect()
	require.NoError(t, err)
	defer socket.Destroy()

	attendances, err := socket.GetAttendances()
	require.NoError(t, err)
	t.Log(len(attendances))
	t.Log(attendances[len(attendances)-1])
}

func TestSocketGetUsers(t *testing.T) {
	socket := NewZkSocket(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(t, socket.Connect())
	defer socket.Destroy()
	_, err := socket.GetUsers()
	require.NoError(t, err)
}

func BenchmarkSocketGetAttendances(b *testing.B) {
	socket := NewZkSocket(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(b, socket.Connect())
	defer socket.Destroy()

	for i := 0; i < b.N; i++ {
		_, err := socket.GetAttendances()
		require.NoError(b, err)
	}
}
