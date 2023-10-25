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

	require.Len(t, status.Sirens, 2)
	for _, siren := range status.Sirens {
		t.Logf("siren: %+v", siren)
	}
	require.Len(t, status.Repeaters, 2)
	for _, repeater := range status.Repeaters {
		t.Logf("repeater: %+v", repeater)
	}

	require.Len(t, status.Partitions, 16)
	for _, part := range status.Partitions {
		t.Logf("partition: %+v", part)
	}

	t.Log(status.BatteryStatus.String())
}

func TestMacAddress(t *testing.T) {
	hw, err := MacAddress("192.168.1.1")
	require.NoError(t, err)
	require.NotEmpty(t, hw)
}
