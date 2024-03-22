package main

import (
	"fmt"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	client "github.com/caarlos0/homekit-amt8000"
)

type Repeater struct {
	*accessory.A
	Connected  *service.ContactSensor
	LowBattery *characteristic.StatusLowBattery
	Tamper     *characteristic.StatusTampered
}

func newRepeater(info accessory.Info) *Repeater {
	a := Repeater{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.Tamper = characteristic.NewStatusTampered()

	a.Connected = service.NewContactSensor()
	a.Connected.AddC(a.Tamper.C)
	a.Connected.AddC(a.LowBattery.C)
	a.AddS(a.Connected.S)

	_ = a.Connected.ContactSensorState.SetValue(0)

	return &a
}

func (repeater *Repeater) Update(status client.Repeater) {
	_ = repeater.LowBattery.SetValue(boolToInt(status.LowBattery))
	_ = repeater.Tamper.SetValue(boolToInt(status.Tamper))
	tamperGauge.WithLabelValues(repeater.Name()).Set(boolToFloat(status.Tamper))
}

func setupRepeaters(cfg Config, status client.Status) []*Repeater {
	var repeaters []*Repeater
	for i, number := range cfg.Repeaters {
		repeater := status.Repeaters[number-1]
		a := newRepeater(accessory.Info{
			Name:         fmt.Sprintf("Repeater %d", number),
			Manufacturer: manufacturer,
		})
		a.Update(repeater)
		a.Id = uint64(300 + i)
		repeaters = append(repeaters, a)
	}
	return repeaters
}
