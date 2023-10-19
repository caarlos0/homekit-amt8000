package isec

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/charmbracelet/log"
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

func (c *Client) TurnOffSiren(partition byte) error {
	log.Debug("turn off siren")
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdTurnOffSiren, []byte{partition})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not turn siren off %v: %w", partition, err)
	}
	return c.recycle()
}

func (c *Client) CleanFirings() error {
	log.Debug("clean firings")
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdCleanFiring, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not clean firing: %w", err)
	}
	return c.recycle()
}

func (c *Client) Status() (OverallStatus, error) {
	log.Debug("status")
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
	return fromBytes(reply), nil
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
	log.Debug("disarm", "partition", partition)
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdDisarm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not disarm: %w", err)
	}
	return c.recycle()
}

func (c *Client) Arm(partition byte) error {
	log.Debug("arm", "partition", partition)
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdArm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not arm %v: %w", partition, err)
	}
	return c.recycle()
}

func (c *Client) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn.Close()
}

func (c *Client) recycle() error {
	log.Debug("recycling client...")
	time.Sleep(time.Second)
	if err := c.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("could not recycle client: %w", err)
	}
	if err := c.init(); err != nil {
		return fmt.Errorf("could not recycle client: %w", err)
	}
	return nil
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

	size, err := payloadSize(payload)
	if err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	resp, err := io.ReadAll(io.LimitReader(c.conn, int64(size)))
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
