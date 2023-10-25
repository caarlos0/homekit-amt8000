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

	return &a
}

func (a *SecuritySystem) Update(cfg Config, status client.Status) {
	if v := cfg.getAlarmState(status); a.SecuritySystem.SecuritySystemCurrentState.Value() != v {
		err := a.SecuritySystem.SecuritySystemCurrentState.SetValue(v)
		log.Info("set current state", "state", v, "err", err)
	}

	if v := boolToInt(status.Tamper); a.Tampered.Value() != v {
		_ = a.Tampered.SetValue(boolToInt(status.Tamper))
		log.Info("alarm status", "tamper", status.Tamper)
	}

	// shows unknown, missing, short-circuit, and dead as a dead battery.
	if v := boolToInt(status.Battery <= client.BatteryStatusDead); a.LowBattery.Value() != v {
		_ = a.LowBattery.SetValue(v)
		log.Info("alarm status", "low-battery", status.Battery.String())
	}

	if v := status.Battery.Level(); a.BatteryLevel.Value() != v {
		_ = a.BatteryLevel.SetValue(v)
		log.Info("alarm status", "battery-level", v)
	}
}
