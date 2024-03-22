package main

import (
	"fmt"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	client "github.com/caarlos0/homekit-amt8000"
)

type Siren struct {
	*accessory.A
	Connected  *service.ContactSensor
	LowBattery *characteristic.StatusLowBattery
	Tamper     *characteristic.StatusTampered
}

func newSiren(info accessory.Info) *Siren {
	a := Siren{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.Tamper = characteristic.NewStatusTampered()

	a.Connected = service.NewContactSensor()
	a.Connected.AddC(a.Tamper.C)
	a.Connected.AddC(a.LowBattery.C)
	a.AddS(a.Connected.S)

	return &a
}

func (siren *Siren) Update(status client.Siren) {
	_ = siren.LowBattery.SetValue(boolToInt(status.LowBattery))
	_ = siren.Tamper.SetValue(boolToInt(status.Tamper))
	tamperGauge.WithLabelValues(siren.Name()).Set(boolToFloat(status.Tamper))
}

func setupSirens(cfg Config, status client.Status) []*Siren {
	var sirens []*Siren
	for i, number := range cfg.Sirens {
		siren := status.Sirens[number-1]
		a := newSiren(accessory.Info{
			Name:         fmt.Sprintf("Siren %d", number),
			Manufacturer: manufacturer,
		})
		a.Update(siren)
		a.Id = uint64(200 + i)
		sirens = append(sirens, a)
	}
	return sirens
}
