package main

import (
	"net/http"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	client "github.com/caarlos0/homekit-amt8000"
)

type AlarmSensor struct {
	*accessory.A
	Motion     *service.MotionSensor
	Contact    *service.ContactSensor
	Bypass     *service.Switch
	LowBattery *characteristic.StatusLowBattery
	Tamper     *characteristic.StatusTampered

	execute Executor
	zone    zoneConfig
}

func newAlarmSensor(info accessory.Info, zone zoneConfig, execute Executor) *AlarmSensor {
	a := &AlarmSensor{
		execute: execute,
		zone:    zone,
	}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.Tamper = characteristic.NewStatusTampered()

	switch zone.kind {
	case kindContact:
		a.Contact = service.NewContactSensor()
		a.Contact.AddC(a.Tamper.C)
		a.Contact.AddC(a.LowBattery.C)
		a.AddS(a.Contact.S)
	case kindMotion:
		a.Motion = service.NewMotionSensor()
		a.Motion.AddC(a.LowBattery.C)
		a.Motion.AddC(a.Tamper.C)
		a.AddS(a.Motion.S)
	}

	a.Bypass = service.NewSwitch()
	a.AddS(a.Bypass.S)
	a.Bypass.On.SetValueRequestFunc = a.updateHandler

	return a
}

func (a *AlarmSensor) updateHandler(
	value interface{},
	_ *http.Request,
) (response interface{}, code int) {
	// we bypass the zone when the switch is ON
	v := !value.(bool)
	log.Info("set zone bypass", "zone", a.zone.number, "bypass", v)
	if err := a.execute(func(cli *client.Client) error {
		return cli.Bypass(a.zone.number, v)
	}); err != nil {
		log.Error("failed to set bypass", "zone", a.zone.number, "value", v, "err", err)
		return nil, hap.JsonStatusResourceBusy
	}
	return nil, hap.JsonStatusSuccess
}

func (a *AlarmSensor) Update(zone client.Zone) {
	batlvl := boolToInt(zone.LowBattery)
	if a.LowBattery.Value() != batlvl {
		log.Info("low battery", "zone", zone.Number, "status", zone.LowBattery)
		_ = a.LowBattery.SetValue(batlvl)
	}

	tamper := boolToInt(zone.Tamper)
	if a.Tamper.Value() != tamper {
		log.Info("tamper", "zone", zone.Number, "status", zone.Tamper)
		_ = a.Tamper.SetValue(tamper)
	}

	bypassing := zone.Anulated
	if a.Bypass.On.Value() == bypassing {
		log.Info("bypass", "zone", zone.Number, "status", !bypassing)
		a.Bypass.On.SetValue(!bypassing)
	}

	switch a.zone.kind {
	case kindContact:
		current := boolToInt(zone.IsOpen())
		if v := a.Contact.ContactSensorState.Value(); v == current {
			return
		}
		_ = a.Contact.ContactSensorState.SetValue(current)
		log.Info(
			"contact",
			"zone", zone.Number,
			"open", zone.Open,
			"violated", zone.Violated,
		)
	case kindMotion:
		current := zone.IsOpen()
		if v := a.Motion.MotionDetected.Value(); v == current {
			return
		}
		a.Motion.MotionDetected.SetValue(current)
		log.Info(
			"motion",
			"zone", zone.Number,
			"open", zone.Open,
			"violated", zone.Violated,
		)
	}
}

func setupZones(
	execute Executor,
	cfg Config,
	status client.Status,
) []*AlarmSensor {
	var sensors []*AlarmSensor
	for _, zone := range cfg.allZones() {
		zone := zone
		a := newAlarmSensor(accessory.Info{
			Name:         zone.name,
			Manufacturer: manufacturer,
		}, zone, execute)
		a.Id = uint64(100 + zone.number)
		a.Update(status.Zones[zone.number])
		sensors = append(sensors, a)
	}
	return sensors
}
