package main

import (
	"fmt"

	"golang.org/x/exp/slices"
)

type Config struct {
	Host             string   `env:"HOST,required"`
	Port             string   `env:"PORT"              envDefault:"9009"`
	Password         string   `env:"PASSWORD,required"`
	MotionZones      []int    `env:"MOTION"`
	ContactZones     []int    `env:"CONTACT"`
	AllowBypassZones []int    `env:"ALLOW_BYPASS"`
	StayPartition    int      `env:"STAY"              envDefault:"1"`
	AwayPartition    int      `env:"AWAY"              envDefault:"0"`
	NightPartition   int      `env:"NIGHT"             envDefault:"2"`
	ZoneNames        []string `env:"ZONE_NAMES"`
}

type zoneKind uint8

const (
	kindMotion = iota + 1
	kindContact
)

type zoneConfig struct {
	number int
	name   string
	kind   zoneKind
}

func (c Config) zoneName(n int) string {
	names := c.ZoneNames
	if len(names) > n-1 {
		if n := names[n-1]; n != "" {
			return n
		}
	}
	return fmt.Sprintf("Zone %d", n)
}

func (c Config) allZones() []zoneConfig {
	var zones []zoneConfig
	for _, z := range c.MotionZones {
		zones = append(zones, zoneConfig{
			number: z,
			name:   c.zoneName(z),
			kind:   kindMotion,
		})
	}
	for _, z := range c.ContactZones {
		zones = append(zones, zoneConfig{
			number: z,
			name:   c.zoneName(z),
			kind:   kindContact,
		})
	}
	slices.SortFunc(zones, func(a, b zoneConfig) int {
		if a.number > b.number {
			return 1
		}
		return -1
	})
	return zones
}
