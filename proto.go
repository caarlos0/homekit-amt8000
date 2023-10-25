package amt8000

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	// 00 = N/A
	// 01 = Remote programming software
	// 02 = Monitoring software
	// 03 = Mobile APP
	// 04 = Clone Account
	deviceType = 0x03

	// version of this software
	softwareVersion = 0x10

	// id to handle responses in the alarm system
	ourID = 0x8ffe

	// no idea
	alarmID = 0x0000
)

func makeAuthPayload(pwd string) []byte {
	contactID, err := contactIDEncode(pwd)
	if err != nil {
		panic(err)
	}
	payload := []byte{deviceType}
	payload = append(payload, contactID...)
	payload = append(payload, softwareVersion)
	return makePayload(cmdAuth, payload)
}

func makePayload(cmd int, input []byte) []byte {
	alarmID := splitIntoOctets(alarmID)
	ourID := splitIntoOctets(ourID)
	length := splitIntoOctets(len(input) + 2)
	cmd_enc := splitIntoOctets(cmd)
	payload := []byte{}
	payload = append(payload, alarmID...)
	payload = append(payload, ourID...)
	payload = append(payload, length...)
	payload = append(payload, cmd_enc...)
	payload = append(payload, input...)
	payload = append(payload, checksum(payload))
	return payload
}

func splitIntoOctets(n int) []byte {
	return []byte{byte(n / 256), byte(n % 256)}
}

func mergeOctets(buf []byte) int {
	return int(buf[0])*256 + int(buf[1])
}

func checksum(buf []byte) byte {
	var check byte
	for _, n := range buf {
		check ^= n
	}
	check ^= 0xff
	check &= 0xff
	return check
}

func contactIDEncode(pwd string) ([]byte, error) {
	var buf []byte
	num, err := strconv.Atoi(pwd)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(pwd); i++ {
		digit, err := strconv.Atoi(string(pwd[i]))
		if err != nil {
			return nil, err
		}
		digit %= 10
		num /= 10
		if digit == 0 {
			digit = 0x0a
		}
		buf = append(buf, byte(digit))
	}

	if len(pwd) == 4 {
		buf = append([]byte{0x0a, 0x0a}, buf...)
	}

	return buf, nil
}

var ErrInvalidPassword = errors.New("invalid password")

func parseAuthResponse(buf []byte) error {
	cmd, result := parseResponse(buf)
	if cmd != 0xf0f0 {
		return fmt.Errorf("invalid command: %v", cmd)
	}
	if len(result) == 0 {
		return fmt.Errorf("invalid response: %v", result)
	}

	switch result[0] {
	case 0:
		return nil
	case 1:
		return ErrInvalidPassword
	default:
		return fmt.Errorf("authentication failed: %v", result[0])
	}
}

func parseResponse(buf []byte) (int, []byte) {
	lenPayload := mergeOctets(buf[4:6]) - 2
	cmd := mergeOctets(buf[6:8])
	if len(buf) < 8+lenPayload || lenPayload < 0 {
		return cmd, nil
	}
	payload := buf[8 : 8+lenPayload]
	return cmd, payload
}
