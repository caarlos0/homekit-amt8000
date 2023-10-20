package isecnetv2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsec(t *testing.T) {
	cli, err := New("192.168.1.111", "9009", "307924")
	require.NoError(t, err)

	require.NoError(t, cli.Bypass(1, false))

	status, err := cli.Status()
	require.NoError(t, err)

	require.Len(t, status.Zones, 48)
	for _, zone := range status.Zones {
		if zone.Open || zone.Violated || zone.Anulated || zone.Tamper || zone.LowBattery ||
			zone.ShortCircuit {
			t.Logf("zone: %+v", zone)
		}
	}

	for _, part := range status.Partitions {
		if part.Stay || part.Armed || part.Fired || part.Firing {
			t.Logf("partition: %+v", part)
		}
	}
}
