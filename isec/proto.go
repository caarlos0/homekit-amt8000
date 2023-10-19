package isec

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
	return createPayload(cmdAuth, payload)
}

func createPayload(cmd int, input []byte) []byte {
	centralID := splitIntoOctets(0x0000)
	ourID := splitIntoOctets(0x8fff)
	length := splitIntoOctets(len(input) + 2)
	cmd_enc := splitIntoOctets(cmd)
	payload := []byte{}
	payload = append(payload, centralID...)
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

func checksum(pacote []byte) byte {
	var check byte
	for _, n := range pacote {
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

func payloadSize(payload []byte) (int64, error) {
	if len(payload) < 9 {
		return 0, fmt.Errorf("wrong payload length: %d", len(payload))
	}
	// TODO: this is likely not correct
	n := mergeOctets(payload[4:6])
	if len(payload) < n {
		return 0, fmt.Errorf("wrong merged payload length: %d < %d", len(payload), n)
	}
	return int64(n), nil
}

func parseResponse(resp []byte) (int, []byte) {
	lenPayload := mergeOctets(resp[4:6]) - 2
	cmd := mergeOctets(resp[6:8])
	if len(resp) < 8+lenPayload || lenPayload < 0 {
		return cmd, nil
	}
	payload := resp[8 : 8+lenPayload]
	return cmd, payload
}
