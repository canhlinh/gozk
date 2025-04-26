package gozk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TODO: Adds more test to the lib
// I'm too lazy to work to this project

const (
	testZkHost   = "192.168.100.201"
	testZkPort   = 4370
	testTimezone = "Asia/Ho_Chi_Minh"
)

func TestSocketConnect(t *testing.T) {
	socket := NewZK(testZkHost, WithTimezone(testTimezone), WithTCP(true))
	require.NoError(t, socket.Connect())
	require.NoError(t, socket.Disconnect())
}

func TestSocketGetAttendances(t *testing.T) {
	socket := NewZK(testZkHost, WithTimezone(testTimezone), WithTCP(true))
	require.NoError(t, socket.Connect())
	require.NoError(t, socket.DisableDevice())

	properties, err := socket.GetProperties()
	require.NoError(t, err)

	attendances, err := socket.GetAllScannedEvents()
	require.NoError(t, err)
	require.Equal(t, properties.TotalRecords, len(attendances))

	require.NoError(t, socket.EnableDevice())
	require.NoError(t, socket.Disconnect())
	time.Sleep(time.Second * 1)
}

func TestSocketGetUsers(t *testing.T) {
	socket := NewZK(testZkHost, WithTimezone(testTimezone), WithTCP(true))
	require.NoError(t, socket.Connect())
	defer socket.Disconnect()
	require.NoError(t, socket.GetUsers())
}

func TestUnlockTheDoor(t *testing.T) {
	socket := NewZK(testZkHost, WithTimezone(testTimezone), WithTCP(true))
	require.NoError(t, socket.Connect())
	defer socket.Disconnect()

	require.NoError(t, socket.UnlockTheDoor(3))
}

func TestWriteLCD(t *testing.T) {
	socket := NewZK(testZkHost, WithTimezone(testTimezone), WithTCP(true))
	require.NoError(t, socket.Connect())
	defer socket.Disconnect()

	require.NoError(t, socket.WriteLCD("Hello world"))
}
