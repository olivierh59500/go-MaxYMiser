package midi

// MachineControl recognizes the native stop/play commands only in complete
// universal real-time MMC messages. Device 7F is the all-call address.
func MachineControl(message []byte) (byte, bool) {
	if len(message) != 6 || message[0] != 0xf0 || message[1] != 0x7f || message[2] > 0x7f || message[3] != 0x06 || message[5] != 0xf7 {
		return 0, false
	}
	if message[4] != 1 && message[4] != 2 {
		return 0, false
	}
	return message[4], true
}
