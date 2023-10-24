package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	client "github.com/caarlos0/homekit-amt8000"
)

type AlarmSensors []*AlarmSensor

func (sensors AlarmSensors) Update(cfg Config, status client.Status) {
	for i, zi := range cfg.allZones() {
		zone := status.Zones[zi.number-1]
		sensor := sensors[i]
		sensor.Update(zone)
	}
}

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

type SecuritySystem struct {
	*accessory.A
	SecuritySystem *service.SecuritySystem
	LowBattery     *characteristic.StatusLowBattery
	BatteryLevel   *characteristic.BatteryLevel
	Tampered       *characteristic.StatusTampered
	BatteryFault   *characteristic.StatusFault
}

func NewSecuritySystem(info accessory.Info) *SecuritySystem {
	a := SecuritySystem{}
	a.A = accessory.New(info, accessory.TypeSecuritySystem)

	a.SecuritySystem = service.NewSecuritySystem()
	a.AddS(a.SecuritySystem.S)

	a.Tampered = characteristic.NewStatusTampered()
	a.SecuritySystem.AddC(a.Tampered.C)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.SecuritySystem.AddC(a.LowBattery.C)

	a.BatteryLevel = characteristic.NewBatteryLevel()
	a.SecuritySystem.AddC(a.BatteryLevel.C)

	a.BatteryFault = characteristic.NewStatusFault()
	a.SecuritySystem.AddC(a.BatteryFault.C)

	return &a
}

func (alarm *SecuritySystem) Update(cfg Config, status client.Status) {
	if state := cfg.getAlarmState(status); alarm.SecuritySystem.SecuritySystemCurrentState.Value() != state {
		err := alarm.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
		log.Info("set current state", "state", state, "err", err)
	}

	_ = alarm.Tampered.SetValue(boolToInt(status.Tamper))
	_ = alarm.BatteryFault.SetValue(
		boolToInt(status.BatteryStatus == client.BatteryStatusMissing),
	)
	_ = alarm.LowBattery.SetValue(
		boolToInt(
			status.BatteryStatus == client.BatteryStatusDead ||
				status.BatteryStatus == client.BatteryStatusMissing,
		),
	)

	switch status.BatteryStatus {
	case client.BatteryStatusMissing,
		client.BatteryStatusDead:
		_ = alarm.BatteryLevel.SetValue(0)
	case client.BatteryStatusLow:
		_ = alarm.BatteryLevel.SetValue(20)
	case client.BatteryStatusMiddle:
		_ = alarm.BatteryLevel.SetValue(50)
	case client.BatteryStatusFull:
		_ = alarm.BatteryLevel.SetValue(100)
	}
}
