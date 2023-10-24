package amt8000

import (
	"encoding/hex"
	"fmt"
	"sync"
)

type Status struct {
	Model         string
	Version       string
	State         State
	ZonesFiring   bool
	ZonesClosed   bool
	Siren         bool
	Tamper        bool
	BatteryStatus BatteryStatus
	Partitions    []Partition
	Zones         []Zone
	Sirens        []Siren
	Repeaters     []Repeater
}

type Zone struct {
	Number     int
	Open       bool
	Violated   bool
	Anulated   bool
	Tamper     bool
	LowBattery bool
}

type Siren struct {
	Number     int
	Tamper     bool
	LowBattery bool
}

type Repeater struct {
	Number     int
	Tamper     bool
	LowBattery bool
}

type BatteryStatus uint8

const (
	BatteryStatusMissing BatteryStatus = iota
	BatteryStatusDead
	BatteryStatusLow
	BatteryStatusMiddle
	BatteryStatusFull
)

// Shows the sensor as open if it either is open or if it is violated.
func (z Zone) IsOpen() bool {
	return z.Open || z.Violated
}

type Partition struct {
	Number int
	Armed  bool
	Fired  bool
	Firing bool
	Stay   bool
}

func fromBytes(resp []byte) (Status, error) {
	if len(resp) != 143 {
		return Status{}, fmt.Errorf("invalid status:\n%s", hex.Dump(resp))
	}
	status := Status{
		Model:       modelName(resp[0]),
		Version:     version(resp[1:4]),
		State:       State(resp[20] >> 5 & 0x03),
		ZonesFiring: resp[20]&0x8 > 0,
		ZonesClosed: resp[20]&0x4 > 0,
		Siren:       resp[20]&0x2 > 0,
		Zones:       make([]Zone, 48),
		Sirens:      make([]Siren, 2),
		Repeaters:   make([]Repeater, 2),
	}

	// partitions
	for i := 0; i < 17; i++ {
		if resp[21+i]&0x80 == 1 {
			continue
		}

		status.Partitions = append(status.Partitions, Partition{
			Number: i,
			Armed:  resp[21+i]&0x01 > 0,
			Firing: resp[21+i]&0x04 > 0,
			Fired:  resp[21+i]&0x08 > 0,
			Stay:   resp[21+i]&0x40 > 0,
		})
	}

	for i := 0; i < 48; i++ {
		status.Zones[i].Number = i + 1
	}

	for i := 0; i < 2; i++ {
		status.Sirens[i].Number = i + 1
		status.Repeaters[i].Number = i + 1
	}

	for i, octet := range resp[38:46] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].Open = true
			}
		}
	}

	for i, octet := range resp[46:53] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].Violated = true
			}
		}
	}

	for i, octet := range resp[54:62] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].Anulated = true
			}
		}
	}

	for i, octet := range resp[89:96] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].Tamper = true
			}
		}
	}

	for i, octet := range resp[104:113] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].LowBattery = true
			}
		}
	}

	// sirens
	for i, octet := range resp[99:101] {
		status.Sirens[i].Tamper = octet&0x01 > 0
	}
	for i, octet := range resp[115:117] {
		status.Sirens[i].LowBattery = octet&0x01 > 0
	}

	// repeaters
	for i, octet := range resp[101:103] {
		status.Repeaters[i].Number = i + 1
		status.Repeaters[i].Tamper = octet&0x01 > 0
	}
	for i, octet := range resp[117:119] {
		status.Repeaters[i].Number = i + 1
		status.Repeaters[i].LowBattery = octet&0x01 > 0
	}

	status.BatteryStatus = batteryStatusFor(resp)
	status.Tamper = resp[71]&(1<<0x01) > 0
	return status, nil
}

var onceBatteryDeadLog sync.Once

func batteryStatusFor(resp []byte) BatteryStatus {
	batt := resp[142]
	switch {
	case batt&0x01 == 0x01:
		return BatteryStatusDead
	case batt&0x02 == 0x02:
		return BatteryStatusLow
	case batt&0x03 == 0x03:
		return BatteryStatusMiddle
	case batt&0x04 == 0x04:
		return BatteryStatusFull
	default:
		onceBatteryDeadLog.Do(func() {
			octet := resp[71]
			if octet&(1<<0x04) > 0 {
				log.Warn("battery is short circuited")
			}
			if octet&(1<<0x05) > 0 {
				log.Warn("battery is missing")
			}
		})
		return BatteryStatusMissing
	}
}
