package isecnetv2

import (
	"encoding/hex"
	"fmt"
)

type Status struct {
	Model       string
	Version     string
	State       State
	ZonesFiring bool
	ZonesClosed bool
	Siren       bool
	Partitions  []Partition
	Zones       []Zone
}

type Zone struct {
	Number   int
	Open     bool
	Violated bool
	Anulated bool
}

// Shows the sensor as open if it either is open or if it is violated.
func (z Zone) IsOpen() bool {
	return z.Open || z.Violated
}

type ZoneEvent uint

const (
	ZoneEventClean ZoneEvent = iota
	ZoneEventOpen
	ZoneEventViolated
	ZoneEventAnulated
)

func (z Zone) AnyEvent() ZoneEvent {
	switch {
	case z.Open:
		return ZoneEventOpen
	case z.Violated:
		return ZoneEventViolated
	case z.Anulated:
		return ZoneEventAnulated
	default:
		return ZoneEventClean
	}
}

type Partition struct {
	Number int
	Armed  bool
	Fired  bool
	Firing bool
	Stay   bool
}

func fromBytes(resp []byte) (Status, error) {
	if len(resp) < 20 {
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
	}

	if len(resp) > 21+17 {
		// partitioning is enabled
		for i := 0; i < 17; i++ {
			// check if partition is disabled
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
	}

	for i := 0; i < 48; i++ {
		status.Zones[i].Number = i + 1
	}

	for i, octet := range resp[38:46] {
		for j := 0; j < 8; j++ {
			if octet&(1<<j) > 0 {
				status.Zones[i+j].Open = true
			}
		}
	}

	for i, octet := range resp[46:54] {
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

	return status, nil
}
