package amt8000

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/caarlos0/sync/cio"
	logp "github.com/charmbracelet/log"
	"github.com/j-keck/arping"
)

var log = logp.NewWithOptions(os.Stderr, logp.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
	Prefix:          "isecnetv2",
})

const timeout = 5 * time.Second

const AllPartitions = 0xff

const (
	cmdAuth         = 0xf0f0
	cmdDisconnect   = 0xf0f1
	cmdStatus       = 0x0b4a
	cmdPanic        = 0x401a
	cmdArm          = 0x401e
	cmdTurnOffSiren = 0x4019
	cmdCleanFiring  = 0x4013
	cmdBypass       = 0x401f
)

type State byte

const (
	StateDisarmed State = 0x00
	StatePartial  State = 0x01
	StateArmed    State = 0x03 // one must ask what 0x02 is... and why its missing...
)

const (
	subCmdDisarm = 0x00
	subCmdArm    = 0x01
	subCmdStay   = 0x02
)

func (s State) String() string {
	switch s {
	case StateDisarmed:
		return "Disarmed"
	case StatePartial:
		return "Partial"
	case StateArmed:
		return "Armed"
	default:
		return "Unknown"
	}
}

type Client struct {
	conn net.Conn
	addr string
	pass string
}

func New(host, port, pass string) (*Client, error) {
	cli := &Client{
		addr: net.JoinHostPort(host, port),
		pass: pass,
	}
	return cli, cli.init()
}

func MacAddress(ip string) (string, error) {
	hw, _, err := arping.Ping(net.ParseIP(ip))
	if err != nil {
		return "", fmt.Errorf("could not get the mac address: %w", err)
	}
	return hw.String(), nil
}

func (c *Client) Panic() error {
	payload := makePayload(cmdPanic, []byte{0x02, 0xa5})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not panic: %w", err)
	}
	return nil
}

func (c *Client) Bypass(zone int, set bool) error {
	// 0x01 add
	// 0x00 remove
	var b byte = 0x00
	if set {
		b = 0x01
	}

	payload := makePayload(cmdBypass, []byte{byte(zone - 1), b})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not set bypass=%v %v: %w", set, zone, err)
	}
	return nil
}

func (c *Client) TurnOffSiren(partition byte) error {
	log.Debug("turn off siren")
	payload := makePayload(cmdTurnOffSiren, []byte{partition})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not turn siren off %v: %w", partition, err)
	}
	return nil
}

func (c *Client) CleanFirings() error {
	log.Debug("clean firings")
	payload := makePayload(cmdCleanFiring, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not clean firing: %w", err)
	}
	return nil
}

func (c *Client) Status() (Status, error) {
	log.Debug("status")
	payload := makePayload(cmdStatus, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", err)
	}

	resp, err := c.limitTimedRead(len(payload))
	if err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", err)
	}

	if len(resp) == 0 {
		return Status{}, fmt.Errorf("buffer is empty")
	}

	resp2, err := c.limitTimedRead(int(resp[0]))
	if err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", err)
	}

	_, reply := parseResponse(append(resp, resp2...))
	return fromBytes(reply)
}

func (c *Client) Disarm(partition byte) error {
	log.Debug("disarm", "partition", partition)
	payload := makePayload(cmdArm, []byte{partition, subCmdDisarm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not disarm: %w", err)
	}
	return nil
}

func (c *Client) Arm(partition byte) error {
	log.Debug("arm", "partition", partition)
	payload := makePayload(cmdArm, []byte{partition, subCmdArm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not arm %v: %w", partition, err)
	}

	resp, err := c.limitTimedRead(6)
	if err != nil {
		return err
	}
	resp, err = c.limitTimedRead(int(resp[5]))
	if err != nil {
		return err
	}

	if resp[0] == 0xf0 {
		return fmt.Errorf("failed to arm: open zones")
	}

	if resp[0] == 0x40 {
		return nil
	}

	return fmt.Errorf("unknown response:\n%s", hex.Dump(resp))
}

func (c *Client) Close() error {
	if _, err := c.conn.Write(makePayload(cmdDisconnect, nil)); err != nil {
		return fmt.Errorf("could not disconnect: %w", err)
	}
	return c.conn.Close()
}

func (c *Client) init() error {
	conn, err := net.DialTimeout("tcp", c.addr, timeout)
	if err != nil {
		return fmt.Errorf("could not connect: %w", err)
	}
	c.conn = conn

	payload := makeAuthPayload(c.pass)
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	resp, err := c.limitTimedRead(10)
	if err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	return parseAuthResponse(resp)
}

func (c *Client) limitTimedRead(n int) ([]byte, error) {
	buf := make([]byte, n)
	m, err := cio.TimeoutReader(c.conn, timeout).Read(buf)
	if err != nil {
		return nil, err
	}
	if m != n {
		return nil, fmt.Errorf("wanted %d bytes, read %d", n, m)
	}
	return buf, nil
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
