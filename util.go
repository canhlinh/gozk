package gozk

import (
	"encoding/hex"
	"fmt"
)

// PrintlHex printls bytes to console as HEX encoding
func PrintlHex(title string, buf []byte) {
	fmt.Printf("%s %q\n", title, hex.EncodeToString(buf))
}
