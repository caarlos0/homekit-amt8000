package isecnetv2

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/caarlos0/sync/cio"
	"github.com/charmbracelet/log"
)

const timeout = 5 * time.Second

const AllPartitions = 0xff

const (
	cmdAuth         = 0xf0f0
	cmdDisconnect   = 0xf0f1
	cmdStatus       = 0x0b4a
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

func (c *Client) Panic() error {
	// TODO: impl
	return nil
}

// adding bypass of zone 1:
// 0000   00 00 00 01 00 04 40 1f 00 01 a4
//
// removing bypass of zone 1:
// 0000   00 00 00 01 00 04 40 1f 00 00 a5
//
// adding bypass of zone 2:
// 0000   00 00 00 01 00 04 40 1f 01 01 a5
//
// removing bypass of zone 2:
// 0000   00 00 00 01 00 04 40 1f 01 00 a4
func (c *Client) Bypass(zone int, set bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// 0x01 add
	// 0x00 remove
	var b byte = 0x00
	if set {
		b = 0x01
	}

	payload := createPayload(cmdBypass, []byte{byte(zone - 1), b})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not set bypass=%v %v: %w", set, zone, c.handleClientError(err))
	}
	return c.recycle()
}

func (c *Client) TurnOffSiren(partition byte) error {
	log.Debug("turn off siren")
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdTurnOffSiren, []byte{partition})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not turn siren off %v: %w", partition, c.handleClientError(err))
	}
	return c.recycle()
}

func (c *Client) CleanFirings() error {
	log.Debug("clean firings")
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdCleanFiring, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not clean firing: %w", c.handleClientError(err))
	}
	return c.recycle()
}

func (c *Client) Status() (Status, error) {
	log.Debug("status")
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdStatus, nil)
	if _, err := c.conn.Write(payload); err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", c.handleClientError(err))
	}

	resp, err := c.limitTimedRead(int64(len(payload)))
	if err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", err)
	}

	resp2, err := c.limitTimedRead(int64(resp[0]))
	if err != nil {
		return Status{}, fmt.Errorf("could not gather status: %w", err)
	}

	_, reply := parseResponse(append(resp, resp2...))
	return fromBytes(reply), nil
}

func (c *Client) Disarm(partition byte) error {
	log.Debug("disarm", "partition", partition)
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdDisarm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not disarm: %w", c.handleClientError(err))
	}
	return c.recycle()
}

func (c *Client) Arm(partition byte) error {
	log.Debug("arm", "partition", partition)
	c.lock.Lock()
	defer c.lock.Unlock()
	payload := createPayload(cmdArm, []byte{partition, subCmdArm})
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("could not arm %v: %w", partition, c.handleClientError(err))
	}
	return c.recycle()
}

func (c *Client) handleClientError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EPIPE) {
		if err := c.recycle(); err != nil {
			return fmt.Errorf(
				"client is broken, and we failed to recycle it: %w",
				err,
			)
		}
	}
	return err
}

func (c *Client) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, err := c.conn.Write(createPayload(cmdDisconnect, nil)); err != nil {
		return fmt.Errorf("could not disconnect: %w", err)
	}
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
		return fmt.Errorf("could not auth: %w", c.handleClientError(err))
	}

	resp, err := c.limitTimedRead(authReplySize(c.pass))
	if err != nil {
		return fmt.Errorf("could not auth: %w", err)
	}

	return parseAuthResponse(resp)
}

func (c *Client) limitTimedRead(n int64) ([]byte, error) {
	return io.ReadAll(cio.TimeoutReader(io.LimitReader(c.conn, n), timeout))
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
