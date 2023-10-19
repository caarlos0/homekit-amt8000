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
	Host         string `env:"HOST,required"`
	Port         string `env:"PORT"              envDefault:"9009"`
	Password     string `env:"PASSWORD,required"`
	MotionZones  []int  `env:"MOTION"`
	ContactZones []int  `env:"CONTACT"`
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

	// Create the switch accessory.
	a := accessory.NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		SerialNumber: "0xf0f0",
		Manufacturer: "Intelbras",
		Model:        status.Model,
		Firmware:     status.Version,
	})
	a.SecuritySystem.SecuritySystemTargetState.OnValueRemoteUpdate(func(v int) {
		switch v {
		case characteristic.SecuritySystemTargetStateStayArm:
			log.Info("arm stay")
			if err := cli.Arm(isec.AllPartitions); err != nil {
				log.Error("could not arm", "err", err)
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			log.Info("arm away")
			if err := cli.Arm(0x02); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			log.Info("arm night")
			if err := cli.Arm(0x02); err != nil {
				log.Error("could not arm partition 2", "err", err)
			}
		case characteristic.SecuritySystemTargetStateDisarm:
			log.Info("disarm")
			if err := cli.Disable(isec.AllPartitions); err != nil {
				log.Error("could not disarm", "err", err)
			}
		}
	})

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
			a.Info.FirmwareRevision.SetValue(status.Version)
			a.Info.Model.SetValue(status.Model)
			// sets the initial state, otherwise it'll keep in "arming" when server restarts
			once.Do(func() {
				state := toCurrentState(status)
				err := a.SecuritySystem.SecuritySystemTargetState.SetValue(state)
				log.Info("set target state", "state", state, "err", err)
			})
			if state := toCurrentState(status); a.SecuritySystem.SecuritySystemCurrentState.Value() != state {
				err := a.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
				log.Info("set current state", "state", state, "err", err)
			}

			for i, zone := range cfg.ContactZones {
				current := boolToInt(status.Zones[zone-1].Open)
				v := contactZones[i].ContactSensor.ContactSensorState.Value()
				if v != current {
					log.Info("contact", "zone", zone, "status", current)
					contactZones[i].ContactSensor.ContactSensorState.SetValue(current)
				}
			}
			for i, zone := range cfg.MotionZones {
				current := status.Zones[zone-1].Open
				v := motionZones[i].MotionSensor.MotionDetected.Value()
				if v != current {
					log.Info("motion", "zone", zone, "status", current)
					motionZones[i].MotionSensor.MotionDetected.SetValue(current)
				}
			}

		}
	}()

	bridge := accessory.New(accessory.Info{
		Name: "Bridge",
	}, accessory.TypeBridge)

	// Store the data in the "./db" directory.
	fs := hap.NewFsStore("./db")

	// Create the hap server.
	server, err := hap.NewServer(fs, bridge, allSensors(a, contactZones, motionZones)...)
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

func toCurrentState(status isec.OverallStatus) int {
	if status.Siren {
		log.Debug("set: firing")
		return characteristic.SecuritySystemCurrentStateAlarmTriggered
	}
	switch status.Status {
	case isec.Armed:
		log.Debug("set: away arm")
		return characteristic.SecuritySystemCurrentStateAwayArm
	case isec.Partial:
		log.Debug("set: night arm")
		return characteristic.SecuritySystemCurrentStateNightArm
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
