package main

import (
	"fmt"

	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
	"golang.org/x/exp/slices"
)

type Config struct {
	Host             string   `env:"HOST,required"`
	Port             string   `env:"PORT"              envDefault:"9009"`
	Password         string   `env:"PASSWORD,required"`
	MotionZones      []int    `env:"MOTION"`
	ContactZones     []int    `env:"CONTACT"`
	AllowBypassZones []int    `env:"ALLOW_BYPASS"`
	AwayPartitions   []int    `env:"AWAY"              envDefault:"0"` // 0xff
	StayPartitions   []int    `env:"STAY"              envDefault:"2"`
	NightPartitions  []int    `env:"NIGHT"             envDefault:"2,3"`
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

func (c Config) getAlarmState(status isecnetv2.Status) int {
	if status.Siren {
		return characteristic.SecuritySystemCurrentStateAlarmTriggered
	}

	switch status.State {
	case isecnetv2.StateDisarmed:
		return characteristic.SecuritySystemCurrentStateDisarmed
	case isecnetv2.StatePartial:
		return c.getPartialStatus(status.Partitions)
	default:
		return c.getArmedState()
	}
}

func (c Config) getArmedState() int {
	if len(c.NightPartitions) == 1 && c.NightPartitions[0] == 0 {
		return characteristic.SecuritySystemCurrentStateNightArm
	}
	if len(c.StayPartitions) == 1 && c.StayPartitions[0] == 0 {
		return characteristic.SecuritySystemCurrentStateStayArm
	}
	if len(c.AwayPartitions) == 1 && c.AwayPartitions[0] == 0 {
		return characteristic.SecuritySystemCurrentStateAwayArm
	}

	return -1
}

func (c Config) getPartialStatus(partitions []isecnetv2.Partition) int {
	armed := []int{}
	for _, part := range partitions {
		log.Debug("partition armed", "part", part.Number, "armed", part.Armed)
		if !part.Armed {
			continue
		}
		armed = append(armed, part.Number)
	}
	if slices.Equal(c.NightPartitions, armed) {
		return characteristic.SecuritySystemCurrentStateNightArm
	}

	if slices.Equal(c.StayPartitions, armed) {
		return characteristic.SecuritySystemCurrentStateStayArm
	}

	if slices.Equal(c.AwayPartitions, armed) {
		return characteristic.SecuritySystemCurrentStateAwayArm
	}

	return -1
}
