package gozk

import "time"

// Option specifies the task processing behavior.
type Option interface {
	// Type describes the type of the option.
	Type() OptionType

	// Value returns a value used to create this option.
	Value() interface{}
}

type OptionType int

const (
	OptionPort OptionType = iota
	OptionPin
	OptionTimezone
	OptionUseTCP
	OptionDeviceID
)

type optionPort int

func (o optionPort) Type() OptionType {
	return OptionPort
}

func (o optionPort) Value() interface{} {
	return int(o)
}

func WithPort(port int) Option {
	return optionPort(port)
}

type optionPin int

func (o optionPin) Type() OptionType {
	return OptionPin
}

func (o optionPin) Value() interface{} {
	return int(o)
}

func WithPin(pin int) Option {
	return optionPin(pin)
}

type optionTimezone string

func (o optionTimezone) Type() OptionType {
	return OptionTimezone
}

func (o optionTimezone) Value() interface{} {
	loc, err := time.LoadLocation(string(o))
	if err != nil {
		panic(err)
	}
	return loc
}

func WithTimezone(tz string) Option {
	return optionTimezone(tz)
}

type optionUseTCP bool

func (o optionUseTCP) Type() OptionType {
	return OptionUseTCP
}

func (o optionUseTCP) Value() interface{} {
	return bool(o)
}

func WithTCP(useTCP bool) Option {
	return optionUseTCP(useTCP)
}

type optionDeviceID string

func (o optionDeviceID) Type() OptionType {
	return OptionDeviceID
}

func (o optionDeviceID) Value() interface{} {
	return string(o)
}

func WithDeviceID(deviceID string) Option {
	return optionDeviceID(deviceID)
}

type option struct {
	port     int
	pin      int
	timezone *time.Location
	useTCP   bool
	deviceID string
	maxChunk int
}

func composeOption(opts ...Option) *option {
	opt := &option{
		port:     4370,
		pin:      0,
		timezone: time.Local,
		useTCP:   true,
		maxChunk: MAX_UDP_CHUNK,
	}

	for _, o := range opts {
		switch o.Type() {
		case OptionPort:
			opt.port = o.Value().(int)
		case OptionPin:
			opt.pin = o.Value().(int)
		case OptionTimezone:
			opt.timezone = o.Value().(*time.Location)
		case OptionUseTCP:
			opt.useTCP = o.Value().(bool)
			if opt.useTCP {
				opt.maxChunk = MAX_TCP_CHUNK
			}
		case OptionDeviceID:
			opt.deviceID = o.Value().(string)
		}
	}

	return opt
}
