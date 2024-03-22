package main

import (
	"testing"

	"github.com/brutella/hap/characteristic"
	client "github.com/caarlos0/homekit-amt8000"
	"github.com/stretchr/testify/require"
)

func TestAllZones(t *testing.T) {
	cfg := Config{
		ContactZones: []int{1, 3, 5, 6, 7},
		MotionZones:  []int{2, 4, 8, 9, 10},
		ZoneNames:    []string{"A", "B", "", "C", "D"},
	}

	zones := cfg.allZones()

	require.Equal(t, []zoneConfig{
		{1, "A", kindContact, false},
		{2, "B", kindMotion, false},
		{3, "Zone 3", kindContact, false},
		{4, "C", kindMotion, false},
		{5, "D", kindContact, false},
		{6, "Zone 6", kindContact, false},
		{7, "Zone 7", kindContact, false},
		{8, "Zone 8", kindMotion, false},
		{9, "Zone 9", kindMotion, false},
		{10, "Zone 10", kindMotion, false},
	}, zones)
}

func TestGetAlarmState(t *testing.T) {
	cfg := Config{
		StayPartitions:  []int{1, 3},
		AwayPartitions:  []int{0},
		NightPartitions: []int{2, 4},
	}

	t.Run("triggered", func(t *testing.T) {
		require.Equal(
			t,
			characteristic.SecuritySystemCurrentStateAlarmTriggered,
			cfg.getAlarmState(client.Status{
				Siren: true,
			}),
		)
	})

	t.Run("night", func(t *testing.T) {
		require.Equal(
			t,
			characteristic.SecuritySystemCurrentStateNightArm,
			cfg.getAlarmState(client.Status{
				State: client.StatePartial,
				Partitions: []client.Partition{
					{
						Number: 1,
						Armed:  false,
					},
					{
						Number: 2,
						Armed:  true,
					},
					{
						Number: 3,
						Armed:  false,
					},
					{
						Number: 4,
						Armed:  true,
					},
				},
			}),
		)
	})

	t.Run("away", func(t *testing.T) {
		require.Equal(
			t,
			characteristic.SecuritySystemCurrentStateAwayArm,
			cfg.getAlarmState(client.Status{
				State: client.StateArmed,
				Partitions: []client.Partition{
					{
						Number: 1,
						Armed:  true,
					},
					{
						Number: 2,
						Armed:  true,
					},
					{
						Number: 3,
						Armed:  true,
					},
					{
						Number: 4,
						Armed:  true,
					},
				},
			}),
		)
	})
}
