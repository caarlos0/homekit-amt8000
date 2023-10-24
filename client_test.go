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

	require.Len(t, status.Zones, 48)
	// for _, zone := range status.Zones {
	// 	if zone.AnyEvent() > ZoneEventClean || zone.Tamper {
	// 		t.Logf("zone: %+v", zone)
	// 	}
	// }

	for _, siren := range status.Sirens {
		t.Log(siren, "siren")
	}
	for _, repeater := range status.Repeaters {
		t.Log(repeater, "repeater")
	}

	// for _, part := range status.Partitions {
	// 	if part.Stay || part.Armed || part.Fired || part.Firing {
	// 		t.Logf("partition: %+v", part)
	// 	}
	// }
}
