package main

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
)

const (
	cmdAuth   = 0xf0f0
	cmdStatus = 0x0b4a
	cmdArm    = 0x401e
)

var statuses = map[byte]string{
	0x00: "Desarmado",
	0x01: "Partial",
	0x03: "Armado",
}

func main() {
	conn, err := net.DialTimeout("tcp", "192.168.1.111:9009", 10*time.Second)
	if err != nil {
		panic(err)
	}
	defer func() { conn.Close() }()
	auth := authPayload("307924")
	_, err = conn.Write(auth)
	if err != nil {
		panic(err)
	}

	fmt.Println("AQUI", packLen(auth))

	resp, err := io.ReadAll(io.LimitReader(conn, int64(packLen(auth))))
	if err != nil {
		panic(err)
	}

	cmd, payload := parseIsecnet2(resp)
	fmt.Println("PARSED", cmd, payload)
	if cmd == 0xf0fd {
		panic("nope 1")
	}
	if cmd != cmdAuth {
		panic("nope 2")
	}

	if len(payload) != 1 {
		panic("nope 3")
	}

	if payload[0] > 0 {
		// 1 incorrect pwd
		// 2 incorrect version
		fmt.Println("PAYLOAD", payload)
		panic("nope 4")
	}

	fmt.Println("AUTH OK")

	payload = createPayload(cmdStatus, nil)
	if _, err := conn.Write(payload); err != nil {
		panic(err)
	}

	resp, err = io.ReadAll(io.LimitReader(conn, 152))
	if err != nil {
		panic(err)
	}

	cmd, resp = parseIsecnet2(resp)

	fmt.Println("status resposnse", cmd, resp)

	// resp = append([]byte{0}, resp...)

	if resp[0] != 0x01 {
		panic("its not an amt800")
	}
	log.Info(
		"central status",
		"model",
		"AMT8000",
		"version",
		versionStr(resp[1:4]),
		"status",
		statuses[resp[20]>>5&0x03],
		"zones-firing",
		resp[20]&0x8 > 0,
		"all-zones-closed",
		resp[20]&0x4 > 0,
		"siren",
		resp[20]&0x2 > 0,
	)

	for i := 0; i < 17; i++ {
		if resp[21+i]&0x80 == 0 {
			// disabled partition
			continue
		}

		armed := resp[21+i]&0x01 > 0
		disparo := resp[21+i]&0x08 > 0
		log.Info("partition", "number", i, "armed", armed, "firing", disparo)
	}

	log.Info("disarming")
	payload = createPayload(cmdArm, []byte{0xff, 0x00})
	if _, err := conn.Write(payload); err != nil {
		panic(err)
	}

	log.Info("arming partition 2")
	payload = createPayload(cmdArm, []byte{0x02, 0x01})
	if _, err := conn.Write(payload); err != nil {
		panic(err)
	}
}

func packLen(payload []byte) int {
	fmt.Println(payload)
	if len(payload) < 9 {
		panic("invalid frame")
	}
	n := parsebe16(payload[4:6])
	if len(payload) < n {
		panic("invalid frame")
	}
	return n
}

func parseIsecnet2(pack []byte) (int, []byte) {
	fmt.Println("AQUI", pack)
	compr_liquido := parsebe16(pack[4:6])
	compr_payload := compr_liquido - 2
	cmd := parsebe16(pack[6:8])
	payload := pack[8 : 8+compr_payload]
	return cmd, payload
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

func authPayload(pwd string) []byte {
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
	centralID := be16(0x0000)
	ourID := be16(0x8fff)
	length := be16(len(input) + 2)
	cmd_enc := be16(cmd)
	payload := []byte{}
	payload = append(payload, centralID...)
	payload = append(payload, ourID...)
	payload = append(payload, length...)
	payload = append(payload, cmd_enc...)
	payload = append(payload, input...)
	payload = append(payload, checksum(payload))
	return payload
}

func be16(n int) []byte {
	return []byte{byte(n / 256), byte(n % 256)}
}

func parsebe16(buf []byte) int {
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

func versionStr(b []byte) string {
	return fmt.Sprintf("%d.%d.%d", int(b[0]), int(b[1]), int(b[2]))
}
