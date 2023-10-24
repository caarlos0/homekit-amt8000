package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	client "github.com/caarlos0/homekit-amt8000"
)

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
