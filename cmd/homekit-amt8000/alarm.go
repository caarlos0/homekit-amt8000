package main

import (
	"net/http"
	"time"

	"github.com/brutella/hap"
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

	cfg     Config
	execute Executor
}

func NewSecuritySystem(info accessory.Info, cfg Config, execute Executor) *SecuritySystem {
	a := &SecuritySystem{
		cfg:     cfg,
		execute: execute,
	}
	a.A = accessory.New(info, accessory.TypeSecuritySystem)

	a.SecuritySystem = service.NewSecuritySystem()
	a.AddS(a.SecuritySystem.S)

	a.Tampered = characteristic.NewStatusTampered()
	a.SecuritySystem.AddC(a.Tampered.C)

	a.LowBattery = characteristic.NewStatusLowBattery()
	a.SecuritySystem.AddC(a.LowBattery.C)

	a.BatteryLevel = characteristic.NewBatteryLevel()
	a.SecuritySystem.AddC(a.BatteryLevel.C)

	a.SecuritySystem.SecuritySystemTargetState.SetValueRequestFunc = a.updateHandler

	return a
}

func (a *SecuritySystem) Update(status client.Status) {
	armStateGauge.Set(float64(a.cfg.getAlarmState(status)))
	tamperedGauge.WithLabelValues("system").Set(boolToFloat(status.Tamper))
	if v := a.cfg.getAlarmState(status); a.SecuritySystem.SecuritySystemCurrentState.Value() != v {
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

func (a *SecuritySystem) updateHandler(
	v interface{},
	_ *http.Request,
) (response interface{}, code int) {
	// If we fail to arm, it might be that some partition succeeded arming,
	// while another didn't...
	// To prevent weird states, we disarm the alarm again if any partition
	// failed.
	disarm := func() {
		_ = a.SecuritySystem.SecuritySystemTargetState.SetValue(
			characteristic.SecuritySystemCurrentStateDisarmed,
		)
		a.updateHandler(characteristic.SecuritySystemCurrentStateDisarmed, nil)
	}

	// Disarm the alarm before any state changes.
	// This allows to properly change between armed states.
	if err := a.execute(func(cli *client.Client) error {
		return cli.Disarm(client.AllPartitions)
	}); err != nil {
		log.Error("could not disarm", "err", err)
		return nil, hap.JsonStatusInvalidValueInRequest
	}

	switch v.(int) {
	case characteristic.SecuritySystemTargetStateStayArm:
		for _, part := range a.cfg.StayPartitions {
			log.Info("arm stay", "partition", part)
			if err := a.execute(func(cli *client.Client) error {
				return cli.Arm(toPartition(part))
			}); err != nil {
				log.Error("could not arm", "err", err)
				disarm()
				return nil, hap.JsonStatusResourceBusy
			}
		}
	case characteristic.SecuritySystemTargetStateAwayArm:
		for _, part := range a.cfg.AwayPartitions {
			log.Info("arm away", "partition", part)
			if err := a.execute(func(cli *client.Client) error {
				return cli.Arm(toPartition(part))
			}); err != nil {
				log.Error("could not arm partition 2", "err", err)
				disarm()
				return nil, hap.JsonStatusResourceBusy
			}
		}
	case characteristic.SecuritySystemTargetStateNightArm:
		for _, part := range a.cfg.NightPartitions {
			log.Info("arm night", "partition", part)
			if err := a.execute(func(cli *client.Client) error {
				return cli.Arm(toPartition(part))
			}); err != nil {
				log.Error("could not arm partition 2", "err", err)
				disarm()
				return nil, hap.JsonStatusResourceBusy
			}
		}
	case characteristic.SecuritySystemTargetStateDisarm:
		log.Info("disarm")
		if a.cfg.CleanFiringsAfter == 0 {
			return nil, hap.JsonStatusSuccess
		}
		go func() {
			time.Sleep(a.cfg.CleanFiringsAfter)
			log.Info("cleaning firings")
			if err := a.execute(func(cli *client.Client) error {
				return cli.CleanFirings()
			}); err != nil {
				log.Error("could not clean firings", "err", err)
			}
		}()
	default:
		return nil, hap.JsonStatusResourceDoesNotExist
	}
	return nil, hap.JsonStatusSuccess
}

func toPartition(i int) byte {
	if i == 0 {
		return client.AllPartitions
	}
	return byte(i)
}
