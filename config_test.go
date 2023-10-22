package main

import (
	"testing"

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
