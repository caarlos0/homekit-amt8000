package main

import (
	"testing"

	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
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
		{1, "A", kindContact},
		{2, "B", kindMotion},
		{3, "Zone 3", kindContact},
		{4, "C", kindMotion},
		{5, "D", kindContact},
		{6, "Zone 6", kindContact},
		{7, "Zone 7", kindContact},
		{8, "Zone 8", kindMotion},
		{9, "Zone 9", kindMotion},
		{10, "Zone 10", kindMotion},
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
			cfg.getAlarmState(isecnetv2.Status{
				Siren: true,
			}),
		)
	})

	t.Run("night", func(t *testing.T) {
		require.Equal(
			t,
			characteristic.SecuritySystemCurrentStateNightArm,
			cfg.getAlarmState(isecnetv2.Status{
				State: isecnetv2.StatePartial,
				Partitions: []isecnetv2.Partition{
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
			characteristic.SecuritySystemCurrentStateNightArm,
			cfg.getAlarmState(isecnetv2.Status{
				State: isecnetv2.StateArmed,
				Partitions: []isecnetv2.Partition{
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
