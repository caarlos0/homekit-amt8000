package amt8000

import (
	"fmt"
	"strconv"
)

func makeAuthPayload(pwd string) []byte {
	softwareType := []byte{0x02}
	softwareVersion := []byte{0x10}
	contactID, err := contactIDEncode(pwd)
	if err != nil {
		panic(err)
	}
	payload := []byte{}
	payload = append(payload, softwareType...)
	payload = append(payload, contactID...)
	payload = append(payload, softwareVersion...)
	return makePayload(cmdAuth, payload)
}

func makePayload(cmd int, input []byte) []byte {
	alarmID := splitIntoOctets(0x0000)
	ourID := splitIntoOctets(0x8ffe)
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
	return buf, nil
}

func authReplySize(pwd string) int {
	switch len(pwd) {
	case 6:
		return 10
	default:
		return 9
	}
}

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
		return fmt.Errorf("invalid password")
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
