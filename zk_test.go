package gozk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TODO: Adds more test to the lib
// I'm too lazy to work to this project

const (
	testZkHost   = "192.168.0.201"
	testZkPort   = 4370
	testTimezone = "Asia/Ho_Chi_Minh"
)

func TestSocketConnect(t *testing.T) {
	socket := NewZK(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(t, socket.Connect())
	require.NoError(t, socket.Disconnect())
}

func TestSocketGetAttendances(t *testing.T) {
	socket := NewZK(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(t, socket.Connect())
	require.NoError(t, socket.DisableDevice())

	attendances, err := socket.GetAttendances()
	require.NoError(t, err)
	t.Log("number of attendances", len(attendances))

	require.NoError(t, socket.EnableDevice())
	require.NoError(t, socket.Disconnect())
	time.Sleep(time.Second * 1)
}

func TestSocketGetUsers(t *testing.T) {
	socket := NewZK(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(t, socket.Connect())
	defer socket.Disconnect()
	require.NoError(t, socket.GetUsers())
}

func BenchmarkSocketGetAttendances(b *testing.B) {
	socket := NewZK(testZkHost, testZkPort, 0, testTimezone)
	require.NoError(b, socket.Connect())
	defer socket.Disconnect()

	for i := 0; i < b.N; i++ {
		_, err := socket.GetAttendances()
		require.NoError(b, err)
	}
}
