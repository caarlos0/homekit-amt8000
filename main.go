package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v9"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
	"github.com/charmbracelet/log"
)

type Config struct {
	Host             string   `env:"HOST,required"`
	Port             string   `env:"PORT"              envDefault:"9009"`
	Password         string   `env:"PASSWORD,required"`
	MotionZones      []int    `env:"MOTION"`
	ContactZones     []int    `env:"CONTACT"`
	AllowBypassZones []int    `env:"ALLOW_BYPASS"`
	StayPartition    int      `env:"STAY"              envDefault:"1"`
	AwayPartition    int      `env:"AWAY"              envDefault:"0"`
	NightPartition   int      `env:"NIGHT"             envDefault:"2"`
	ZoneNames        []string `env:"ZONE_NAMES"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("could not parse env", "err", err)
	}

	cli, err := isecnetv2.New(cfg.Host, cfg.Port, cfg.Password)
	if err != nil {
		log.Fatal("could not init isecnet2 client", "err", err)
	}
	defer func() {
		if err := cli.Close(); err != nil {
			log.Error("could not close isecnet2 client", "err", err)
		}
	}()

	status, err := cli.Status()
	if err != nil {
		log.Fatal("could not init isecnet2 homebridge", "err", err)
	}

	log.Info(
		"bridge configurations:",
		"stay_partition", cfg.StayPartition,
		"away_partition", cfg.AwayPartition,
		"night_partition", cfg.NightPartition,
		"motion_sensor_zones", cfg.MotionZones,
		"contact_sensor_zones", cfg.ContactZones,
	)

	bridge := accessory.NewBridge(accessory.Info{
		Name: "Alarm Bridge",
	})

	alarm := accessory.NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		Manufacturer: "Intelbras",
		Model:        status.Model,
		Firmware:     status.Version,
	})
	alarm.SecuritySystem.SecuritySystemTargetState.OnValueRemoteUpdate(alarmUpdateHandler(cli, cfg))

	contacts, motions, bypasses := setupZones(cli, cfg, status)

	go func() {
		var once sync.Once
		tick := time.NewTicker(time.Second * 3)
		for range tick.C {
			if cli == nil {
				continue
			}
			status, err := cli.Status()
			if err != nil {
				log.Error("could not get status", "err", err)
				continue
			}
			alarm.Info.FirmwareRevision.SetValue(status.Version)
			alarm.Info.Model.SetValue(status.Model)
			// sets the initial state, otherwise it'll keep in "arming" when server restarts
			once.Do(func() {
				if state := toCurrentState(cfg, status); state >= 0 {
					err := alarm.SecuritySystem.SecuritySystemTargetState.SetValue(state)
					log.Info("set target state", "state", state, "err", err)
				}
			})

			if state := toCurrentState(cfg, status); state >= 0 &&
				alarm.SecuritySystem.SecuritySystemCurrentState.Value() != state {
				err := alarm.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
				log.Info("set current state", "state", state, "err", err)
			}

			for i, zone := range cfg.AllowBypassZones {
				current := status.Zones[zone-1].Anulated
				if v := bypasses[i].Switch.On.Value(); v == current {
					continue
				}
				bypasses[i].Switch.On.SetValue(current)
				log.Info("contact", "zone", zone, "status", current)
			}
			for i, zone := range cfg.ContactZones {
				current := boolToInt(status.Zones[zone-1].Open)
				if v := contacts[i].ContactSensor.ContactSensorState.Value(); v == current {
					continue
				}
				_ = contacts[i].ContactSensor.ContactSensorState.SetValue(current)
				log.Info("contact", "zone", zone, "status", current)
			}
			for i, zone := range cfg.MotionZones {
				current := status.Zones[zone-1].Open
				if v := motions[i].MotionSensor.MotionDetected.Value(); v == current {
					continue
				}
				motions[i].MotionSensor.MotionDetected.SetValue(current)
				log.Info("motion", "zone", zone, "status", current)
			}
		}
	}()

	// Store the data in the "./db" directory.
	fs := hap.NewFsStore("./db")

	// Create the hap server.
	server, err := hap.NewServer(
		fs,
		bridge.A,
		securityAccessories(alarm, contacts, motions, bypasses)...,
	)
	if err != nil {
		// stop if an error happens
		log.Fatal("fail", "error", err)
	}

	// Setup a listener for interrupts and SIGTERM signals
	// to stop the server.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		log.Info("stopping server...")
		signal.Stop(c)
		cancel()
	}()

	// Run the server.
	log.Info("starting server...")
	if err := server.ListenAndServe(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("failed to close server", "err", err)
	}
}

func toCurrentState(cfg Config, status isecnetv2.OverallStatus) int {
	if status.Siren {
		log.Debug("set: firing")
		return characteristic.SecuritySystemCurrentStateAlarmTriggered
	}

	switch status.State {
	case isecnetv2.StatePartial, isecnetv2.StateArmed:
		for _, part := range status.Partitions {
			log.Debug("partition armed", "part", part.Number, "armed", part.Armed)
			if !part.Armed {
				continue
			}
			switch part.Number {
			case cfg.NightPartition:
				log.Debug("set: night arm")
				return characteristic.SecuritySystemCurrentStateNightArm
			case cfg.AwayPartition:
				log.Debug("set: away arm")
				return characteristic.SecuritySystemCurrentStateAwayArm
			case cfg.StayPartition:
				log.Debug("set: stay arm")
				return characteristic.SecuritySystemCurrentStateStayArm
			default:
				log.Warn(
					"partition is armed, but its not configured for any state",
					"partition",
					part.Number,
				)
			}
		}

		log.Debug("set: none")
		return -1
	default:
		log.Debug("set: disarm")
		return characteristic.SecuritySystemCurrentStateDisarmed
	}
}

func securityAccessories(
	alarm *accessory.SecuritySystem,
	contacts []*ContactSensor,
	motions []*MotionSensor,
	bypasses []*accessory.Switch,
) []*accessory.A {
	result := []*accessory.A{alarm.A}
	for _, c := range contacts {
		result = append(result, c.A)
	}
	for _, m := range motions {
		result = append(result, m.A)
	}
	for _, m := range bypasses {
		result = append(result, m.A)
	}
	return result
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func zoneName(cfg Config, n int, s string) string {
	names := cfg.ZoneNames
	if len(names) > n-1 {
		return fmt.Sprintf("%s %s", names[n-1], s)
	}
	return fmt.Sprintf("Zone %d %s", n, s)
}

func setupZones(
	cli *isecnetv2.Client,
	cfg Config,
	status isecnetv2.OverallStatus,
) ([]*ContactSensor, []*MotionSensor, []*accessory.Switch) {
	contacts := make([]*ContactSensor, len(cfg.ContactZones))
	motions := make([]*MotionSensor, len(cfg.MotionZones))
	bypasses := make([]*accessory.Switch, len(cfg.AllowBypassZones))
	for i, zone := range cfg.ContactZones {
		a := newContactSensor(accessory.Info{
			Name:         zoneName(cfg, zone, "sensor"),
			Manufacturer: "Intelbras",
		})
		if status.Zones[zone-1].Open {
			_ = a.ContactSensor.ContactSensorState.SetValue(1)
		}
		contacts[i] = a
	}
	for i, zone := range cfg.MotionZones {
		a := newMotionSensor(accessory.Info{
			Name:         zoneName(cfg, zone, "sensor"),
			Manufacturer: "Intelbras",
		})
		if status.Zones[zone-1].Open {
			a.MotionSensor.MotionDetected.SetValue(true)
		}
		motions[i] = a
	}

	for i, zone := range cfg.AllowBypassZones {
		zone := zone
		a := accessory.NewSwitch(accessory.Info{
			Name:         zoneName(cfg, zone, "bypass"),
			Manufacturer: "Intelbras",
		})
		a.Switch.On.OnValueRemoteUpdate(func(v bool) {
			if err := cli.Bypass(zone, v); err != nil {
				log.Error("failed to set bypass", "zone", zone, "value", v, "err", err)
			}
		})
		if status.Zones[zone-1].Anulated {
			a.Switch.On.SetValue(true)
		}
		bypasses[i] = a
	}

	return contacts, motions, bypasses
}

func alarmUpdateHandler(cli *isecnetv2.Client, cfg Config) func(v int) {
	return func(v int) {
		switch v {
		case characteristic.SecuritySystemTargetStateStayArm:
			log.Info("arm stay")
			if err := cli.Arm(toPartition(cfg.StayPartition)); err != nil {
				log.Error("could not arm", "err", err)
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			log.Info("arm away")
			if err := cli.Arm(toPartition(cfg.AwayPartition)); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			log.Info("arm night")
			if err := cli.Arm(toPartition(cfg.NightPartition)); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateDisarm:
			log.Info("disarm")
			if err := cli.Disarm(isecnetv2.AllPartitions); err != nil {
				log.Error("could not disarm", "err", err)
			}
		}
	}
}

func toPartition(i int) byte {
	if i == 0 {
		return isecnetv2.AllPartitions
	}
	return byte(i)
}
