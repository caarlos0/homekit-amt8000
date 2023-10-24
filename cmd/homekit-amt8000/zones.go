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
	Kind       zoneKind
	Motion     *service.MotionSensor
	Contact    *service.ContactSensor
	Bypass     *service.Switch
	LowBattery *characteristic.StatusLowBattery
	Tamper     *characteristic.StatusTampered
}

func (sensor *AlarmSensor) Update(zone client.Zone) {
	batlvl := boolToInt(zone.LowBattery)
	if sensor.LowBattery.Value() != batlvl {
		log.Info("low battery", "zone", zone.Number, "status", zone.LowBattery)
		_ = sensor.LowBattery.SetValue(batlvl)
	}

	tamper := boolToInt(zone.Tamper)
	if sensor.Tamper.Value() != tamper {
		log.Info("tamper", "zone", zone.Number, "status", zone.Tamper)
		_ = sensor.Tamper.SetValue(tamper)
	}

	bypassing := zone.Anulated
	if sensor.Bypass.On.Value() == bypassing {
		log.Info("bypass", "zone", zone.Number, "status", !bypassing)
		sensor.Bypass.On.SetValue(!bypassing)
	}

	switch sensor.Kind {
	case kindContact:
		current := boolToInt(zone.IsOpen())
		if v := sensor.Contact.ContactSensorState.Value(); v == current {
			return
		}
		_ = sensor.Contact.ContactSensorState.SetValue(current)
		log.Info(
			"contact",
			"zone", zone.Number,
			"status", current,
			"open", zone.Open,
			"violated", zone.Violated,
		)
	case kindMotion:
		current := zone.IsOpen()
		if v := sensor.Motion.MotionDetected.Value(); v == current {
			return
		}
		sensor.Motion.MotionDetected.SetValue(current)
		log.Info(
			"motion",
			"zone", zone.Number,
			"status", current,
			"open", zone.Open,
			"violated", zone.Violated,
		)
	}
}

func newAlarmSensor(info accessory.Info, kind zoneKind) *AlarmSensor {
	a := AlarmSensor{
		Kind: kind,
	}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.Tamper = characteristic.NewStatusTampered()

	switch kind {
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

	return &a
}

func setupZones(
	cli clientProvider,
	cfg Config,
	status client.Status,
) []*AlarmSensor {
	var sensors []*AlarmSensor
	for _, zone := range cfg.allZones() {
		zone := zone

		bypassFn := func(value interface{}, _ *http.Request) (response interface{}, code int) {
			v := value.(bool)
			log.Info("set zone bypass", "zone", zone.number, "bypass", v)
			if err := cli(func(cli *client.Client) error {
				return cli.Bypass(zone.number, !v)
			}); err != nil {
				log.Error("failed to set bypass", "zone", zone.number, "value", v, "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
			return nil, hap.JsonStatusSuccess
		}

		a := newAlarmSensor(accessory.Info{
			Name:         zone.name,
			Manufacturer: manufacturer,
		}, zone.kind)
		a.Id = uint64(100 + zone.number)

		a.Bypass.On.SetValueRequestFunc = bypassFn
		a.Update(status.Zones[zone.number])

		sensors = append(sensors, a)
	}
	return sensors
}
