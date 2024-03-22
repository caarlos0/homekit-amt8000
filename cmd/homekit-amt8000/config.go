package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/brutella/hap/characteristic"
	client "github.com/caarlos0/homekit-amt8000"
	"golang.org/x/exp/slices"
)

type Config struct {
	Host              string        `env:"HOST,notEmpty"`
	Port              string        `env:"PORT"                envDefault:"9009"`
	Password          string        `env:"PASSWORD,notEmpty"`
	MotionZones       []int         `env:"MOTION"`
	ContactZones      []int         `env:"CONTACT"`
	BypassZones       []int         `env:"BYPASS"`
	AwayPartitions    []int         `env:"AWAY,notEmpty"`
	StayPartitions    []int         `env:"STAY,notEmpty"`
	NightPartitions   []int         `env:"NIGHT,notEmpty"`
	ZoneNames         []string      `env:"ZONE_NAMES"`
	Sirens            []int         `env:"SIRENS"`
	Repeaters         []int         `env:"REPEATERS"`
	CleanFiringsAfter time.Duration `env:"CLEAN_FIRINGS_AFTER"`
	Address           string        `env:"LISTEN" envDefault:":9009"`
}

type zoneKind uint8

const (
	kindMotion = iota + 1
	kindContact
)

func (z zoneKind) String() string {
	switch z {
	case kindMotion:
		return "motion"
	default:
		return "contact"
	}
}

type zoneConfig struct {
	number      int
	name        string
	kind        zoneKind
	allowBypass bool
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

type allZoneConfigs []zoneConfig

func (a allZoneConfigs) String() string {
	var zones []string
	for _, zone := range a {
		zones = append(
			zones,
			fmt.Sprintf("zone %d: %q (%s)", zone.number, zone.name, zone.kind.String()),
		)
	}
	return strings.Join(zones, "\n")
}

func (c Config) allZones() []zoneConfig {
	var zones []zoneConfig
	for _, z := range c.MotionZones {
		zones = append(zones, zoneConfig{
			number:      z,
			name:        c.zoneName(z),
			kind:        kindMotion,
			allowBypass: slices.Contains(c.BypassZones, z),
		})
	}
	for _, z := range c.ContactZones {
		zones = append(zones, zoneConfig{
			number:      z,
			name:        c.zoneName(z),
			kind:        kindContact,
			allowBypass: slices.Contains(c.BypassZones, z),
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

func (c Config) getAlarmState(status client.Status) int {
	if status.Siren {
		return characteristic.SecuritySystemCurrentStateAlarmTriggered
	}

	switch status.State {
	case client.StateDisarmed:
		return characteristic.SecuritySystemCurrentStateDisarmed
	case client.StatePartial:
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

func (c Config) getPartialStatus(partitions []client.Partition) int {
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
