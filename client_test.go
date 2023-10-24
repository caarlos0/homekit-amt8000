package amt8000

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsec(t *testing.T) {
	cli, err := New("192.168.1.111", "9009", "307924")
	require.NoError(t, err)
	t.Cleanup(func() {
		cli.Close()
	})

	status, err := cli.Status()
	require.NoError(t, err)

	require.Len(t, status.Zones, 64)
	for _, zone := range status.Zones {
		t.Logf("zone: %+v", zone)
	}

	for _, siren := range status.Sirens {
		t.Logf("siren: %+v", siren)
	}
	for _, repeater := range status.Repeaters {
		t.Logf("repeater: %+v", repeater)
	}

	for _, part := range status.Partitions {
		t.Logf("partition: %+v", part)
	}
}

func TestMacAddress(t *testing.T) {
	cli := &Client{
		addr: "192.168.1.111",
	}
	hw, err := cli.HWAddress()
	require.NoError(t, err)
	require.NotEmpty(t, hw)
}
