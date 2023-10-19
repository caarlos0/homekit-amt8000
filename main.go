package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/amt8000-homebridge/isec"
	"github.com/caarlos0/env/v9"
	"github.com/charmbracelet/log"
)

type Config struct {
	Host           string `env:"HOST,required"`
	Port           string `env:"PORT"              envDefault:"9009"`
	Password       string `env:"PASSWORD,required"`
	MotionZones    []int  `env:"MOTION"`
	ContactZones   []int  `env:"CONTACT"`
	StayPartition  byte   `env:"STAY"              envDefault:"1"`
	AwayPartition  byte   `env:"AWAY"              envDefault:"255"`
	NightPartition byte   `env:"NIGHT"             envDefault:"2"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("could not parse env", "err", err)
	}

	cli, err := isec.New(cfg.Host, cfg.Port, cfg.Password)
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
		"partitions configuration:",
		"stay",
		cfg.StayPartition,
		"away",
		cfg.AwayPartition,
		"night",
		cfg.NightPartition,
	)

	bridge := accessory.New(accessory.Info{
		Name: "Alarm Bridge",
	}, accessory.TypeBridge)

	alarm := accessory.NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		Manufacturer: "Intelbras",
		Model:        status.Model,
		Firmware:     status.Version,
	})
	alarm.SecuritySystem.SecuritySystemTargetState.OnValueRemoteUpdate(alarmUpdateHandler(cli, cfg))

	contactZones, motionZones := setupZones(cfg, status)

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

			for i, zone := range cfg.ContactZones {
				current := boolToInt(status.Zones[zone-1].Open)
				v := contactZones[i].ContactSensor.ContactSensorState.Value()
				if v != current {
					contactZones[i].ContactSensor.ContactSensorState.SetValue(current)
					log.Info("contact", "zone", zone, "status", current)
				}
			}
			for i, zone := range cfg.MotionZones {
				current := status.Zones[zone-1].Open
				v := motionZones[i].MotionSensor.MotionDetected.Value()
				if v != current {
					motionZones[i].MotionSensor.MotionDetected.SetValue(current)
					log.Info("motion", "zone", zone, "status", current)
				}
			}

		}
	}()

	// Store the data in the "./db" directory.
	fs := hap.NewFsStore("./db")

	// Create the hap server.
	server, err := hap.NewServer(fs, bridge, allSensors(alarm, contactZones, motionZones)...)
	if err != nil {
		// stop if an error happens
		log.Fatal("fail", "error", err)
	}

	// Setup a listener for interrupts and SIGTERM signals
	// to stop the server.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		// Stop delivering signals.
		signal.Stop(c)
		// Cancel the context to stop the server.
		cancel()
	}()

	// Run the server.
	log.Info("starting server...")
	server.ListenAndServe(ctx)
}

func toCurrentState(cfg Config, status isec.OverallStatus) int {
	if status.Siren {
		log.Debug("set: firing")
		return characteristic.SecuritySystemCurrentStateAlarmTriggered
	}

	switch status.Status {
	case isec.Partial, isec.Armed:
		for _, part := range status.Partitions {
			log.Debug("partition armed", "part", part.Number, "armed", part.Armed)
			if !part.Armed {
				continue
			}
			switch toPartition(part.Number) {
			case cfg.NightPartition:
				log.Debug("set: away night")
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

func allSensors(
	alarm *accessory.SecuritySystem,
	contacts []*ContactSensor,
	motions []*MotionSensor,
) []*accessory.A {
	result := []*accessory.A{alarm.A}
	for _, c := range contacts {
		if c == nil {
			log.Warn("nil")
			continue
		}
		result = append(result, c.A)
	}
	for _, m := range motions {
		if m == nil {
			log.Warn("nil")
			continue
		}
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

func setupZones(cfg Config, status isec.OverallStatus) ([]*ContactSensor, []*MotionSensor) {
	contactZones := make([]*ContactSensor, len(cfg.ContactZones))
	motionZones := make([]*MotionSensor, len(cfg.MotionZones))
	for i, zone := range cfg.ContactZones {
		sensor := newContactSensor(accessory.Info{
			Name:         fmt.Sprintf("Zone %d", zone),
			Manufacturer: "Intelbras",
		})
		if status.Zones[zone-1].Open {
			sensor.ContactSensor.ContactSensorState.SetValue(1)
		}
		contactZones[i] = sensor
	}
	for i, zone := range cfg.MotionZones {
		sensor := newMotionSensor(accessory.Info{
			Name:         fmt.Sprintf("Zone %d", zone),
			Manufacturer: "Intelbras",
		})
		if status.Zones[zone-1].Open {
			sensor.MotionSensor.MotionDetected.SetValue(true)
		}
		motionZones[i] = sensor
	}

	return contactZones, motionZones
}

func alarmUpdateHandler(cli *isec.Client, cfg Config) func(v int) {
	return func(v int) {
		switch v {
		case characteristic.SecuritySystemTargetStateStayArm:
			log.Info("arm stay")
			if err := cli.Arm(cfg.StayPartition); err != nil {
				log.Error("could not arm", "err", err)
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			log.Info("arm away")
			if err := cli.Arm(cfg.AwayPartition); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			log.Info("arm night")
			if err := cli.Arm(cfg.NightPartition); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateDisarm:
			log.Info("disarm")
			if err := cli.Disable(isec.AllPartitions); err != nil {
				log.Error("could not disarm", "err", err)
			}
		}
	}
}

func toPartition(i int) byte {
	if i == 0 {
		return isec.AllPartitions
	}
	return byte(i)
}
