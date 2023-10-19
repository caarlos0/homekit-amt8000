package isec

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const timeout = 15 * time.Second

const AllPartitions = 0xff

const (
	cmdAuth         = 0xf0f0
	cmdStatus       = 0x0b4a
	cmdArm          = 0x401e
	cmdTurnOffSiren = 0x4019
	cmdCleanFiring  = 0x4013
)

type State byte

const (
	Disarmed State = 0x00
	Partial  State = 0x01
	Armed    State = 0x03
)

const (
	subCmdDisarm = 0x00
	subCmdArm    = 0x01
	subCmdStay   = 0x02
)

func (s State) String() string {
	switch s {
	case Disarmed:
		return "Unarmed"
	case Partial:
		return "Partial"
	case Armed:
		return "Armed"
	default:
		return "Unknown"
	}
}

type Client struct {
	lock sync.Mutex
	conn net.Conn
}

func New(host, port, pass string) (*Client, error) {
	cli := &Client{}
	if err := cli.init(host, port); err != nil {
		return nil, err
	}
	if err := cli.auth(pass); err != nil {
		return nil, err
	}
	return cli, nil
}

type OverallStatus struct {
	Model       string
	Version     string
	Status      State
	ZonesFiring bool
	ZonesClosed bool
	Siren       bool
	Partitions  []Partition
}

type Partition struct {
	Number int
	Armed  bool
	Fired  bool
	Firing bool
	Stay   bool
}

func (c *Client) TurnOffSiren(partition byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdTurnOffSiren, []byte{partition})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not turn siren off %v: %w", partition, err)
	}
	return nil
}

func (c *Client) CleanFirings() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdCleanFiring, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not clean firing: %w", err)
	}
	return nil
}

func (c *Client) Status() (OverallStatus, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdStatus, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return OverallStatus{}, fmt.Errorf("could not gather status: %w", err)
	}

	resp, err := io.ReadAll(io.LimitReader(c.conn, 152))
	if err != nil {
		return OverallStatus{}, fmt.Errorf("could not gather status: %w", err)
	}

	_, reply := parseResponse(resp)
	if len(reply) < 21 {
		return OverallStatus{}, fmt.Errorf("alarm between changing states right now")
	}
	result := OverallStatus{
		Model:       modelName(reply[0]),
		Version:     version(reply[1:4]),
		Status:      State(reply[20] >> 5 & 0x03),
		ZonesFiring: reply[20]&0x8 > 0,
		ZonesClosed: reply[20]&0x4 > 0,
		Siren:       reply[20]&0x2 > 0,
	}

	for i := 0; i < 17; i++ {
		// check if partition is disabled
		if resp[21+i]&0x80 == 0 {
			continue
		}

		result.Partitions = append(result.Partitions, Partition{
			Number: i,
			Armed:  reply[21+i]&0x01 > 0,
			Firing: reply[21+i]&0x04 > 0,
			Fired:  reply[21+i]&0x08 > 0,
			Stay:   reply[21+i]&0x40 > 0,
		})
	}

	return result, nil
}

func version(b []byte) string {
	return fmt.Sprintf("%d.%d.%d", int(b[0]), int(b[1]), int(b[2]))
}

func modelName(b byte) string {
	switch b {
	case 0x01:
		return "AMT-8000"
	default:
		return "Unknown"
	}
}

func (c *Client) Disable(partition byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdDisarm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not disarm: %w", err)
	}
	return nil
}

func (c *Client) Arm(partition byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdArm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not arm %v: %w", partition, err)
	}
	return nil
}

func (c *Client) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn.Close()
}

func (c *Client) init(host, port string) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return fmt.Errorf("could not connect: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) auth(pass string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := makeAuthPayload(pass)
	_, err := c.conn.Write(payload)
	if err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	resp, err := io.ReadAll(io.LimitReader(c.conn, int64(payloadSize(payload))))
	if err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	cmd, result := parseResponse(resp)
	if cmd != cmdAuth || len(result) != 1 {
		return fmt.Errorf(
			"could not auth: cmd is not auth, payload size is wrong: %v %v",
			cmd,
			result,
		)
	}
	return nil
}

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

func payloadSize(payload []byte) int {
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

func parseResponse(resp []byte) (int, []byte) {
	lenPayload := parsebe16(resp[4:6]) - 2
	cmd := parsebe16(resp[6:8])
	if len(resp) < 8+lenPayload || lenPayload < 0 {
		return cmd, nil
	}
	payload := resp[8 : 8+lenPayload]
	return cmd, payload
}
