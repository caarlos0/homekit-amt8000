package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/isecnet2/isec"
	"github.com/charmbracelet/log"
)

func newCli() *isec.Client {
	cli, err := isec.New("192.168.1.111", "9009", "307924")
	if err != nil {
		log.Fatal("could not init amt8000", "err", err)
	}
	return cli
}

func main() {
	cli := newCli()
	defer func() { _ = cli.Close() }()

	// Create the switch accessory.
	a := accessory.NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		SerialNumber: "0xf0f0",
		Manufacturer: "Intelbras",
		Model:        "AMT8000",
		Firmware:     "0.0.0",
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
		_ = cli.Close()
		cli = nil
		time.Sleep(time.Second)
		cli = newCli()
	})

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
				a.SecuritySystem.SecuritySystemTargetState.SetValue(toCurrentState(status))
			})
			if state := toCurrentState(status); a.SecuritySystem.SecuritySystemCurrentState.Value() != state {
				log.Info("set", "state", state)
				a.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
			}

		}
	}()

	// Store the data in the "./db" directory.
	fs := hap.NewFsStore("./db")

	// Create the hap server.
	server, err := hap.NewServer(fs, a.A)
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
