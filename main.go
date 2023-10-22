package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v9"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
	logp "github.com/charmbracelet/log"
)

var log = logp.NewWithOptions(os.Stderr, logp.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
	Prefix:          "homekit",
})

type clientProvider = func(func(cli *isecnetv2.Client) error) error

const manufacturer = "Intelbras"

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("could not parse env", "err", err)
	}

	log.Info(
		"loading accessories",
		"configuration", strings.Join([]string{
			fmt.Sprintf("stay partition %d", cfg.StayPartitions),
			fmt.Sprintf("away_partition %d", cfg.AwayPartitions),
			fmt.Sprintf("night_partition %d", cfg.NightPartitions),
			fmt.Sprintf("motion sensors zones %v", cfg.MotionZones),
			fmt.Sprintf("contact sensors zones %v", cfg.ContactZones),
			fmt.Sprintf("zones with bypass %v", cfg.AllowBypassZones),
			fmt.Sprintf("zone names %v", cfg.ZoneNames),
		},
			"\n",
		),
	)

	var clientLock sync.Mutex
	withCli := func(fn func(cli *isecnetv2.Client) error) error {
		t := time.Now()
		clientLock.Lock()
		defer clientLock.Unlock()
		log.Debugf("got client lock after %s", time.Since(t))

		cli, err := isecnetv2.New(cfg.Host, cfg.Port, cfg.Password)
		if err != nil {
			return fmt.Errorf("could not init isecnet2 client: %w", err)
		}
		defer func() {
			if err := cli.Close(); err != nil {
				log.Error("could not close isecnet2 client", "err", err)
			}
		}()
		return fn(cli)
	}

	var status isecnetv2.Status
	if err := withCli(func(cli *isecnetv2.Client) (err error) {
		status, err = cli.Status()
		return
	}); err != nil {
		log.Fatal("could not init accessories", "err", err)
	}

	bridge := accessory.NewBridge(accessory.Info{
		Name: "Alarm Bridge",
	})

	alarm := accessory.NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		Manufacturer: manufacturer,
		Model:        status.Model,
		Firmware:     status.Version,
	})
	if state := cfg.getAlarmState(status); state >= 0 {
		err := alarm.SecuritySystem.SecuritySystemTargetState.SetValue(state)
		log.Info("set target state", "state", state, "err", err)
	}
	alarm.SecuritySystem.SecuritySystemTargetState.SetValueRequestFunc = alarmUpdateHandler(
		withCli,
		cfg,
	)

	contacts, motions := setupZones(withCli, cfg, status)

	panicBtn := accessory.NewSwitch(accessory.Info{
		Name:         "Trigger panic",
		Manufacturer: manufacturer,
	})
	panicBtn.Switch.On.SetValueRequestFunc = func(value interface{}, _ *http.Request) (response interface{}, code int) {
		v := value.(bool)
		if err := withCli(func(cli *isecnetv2.Client) error {
			if v {
				log.Warn("triggering a panic!")
				return cli.Panic()
			}
			return cli.Disarm(isecnetv2.AllPartitions)
		}); err != nil {
			log.Error("failed to trigger panic", "err", err)
			return nil, hap.JsonStatusResourceBusy
		}
		return nil, hap.JsonStatusSuccess
	}

	go func() {
		tick := time.NewTicker(time.Second * 3)
		for range tick.C {
			var status isecnetv2.Status
			if err := withCli(func(cli *isecnetv2.Client) (err error) {
				status, err = cli.Status()
				return
			}); err != nil {
				log.Error("could not get status", "err", err)
				continue
			}

			if state := cfg.getAlarmState(status); alarm.SecuritySystem.SecuritySystemCurrentState.Value() != state {
				err := alarm.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
				log.Info("set current state", "state", state, "err", err)
			}

			ContactSensors(contacts).Update(cfg, status)
			MotionSensors(motions).Update(cfg, status)
			panicBtn.Switch.On.SetValue(status.Siren)
		}
	}()

	fs := hap.NewFsStore("./db")

	server, err := hap.NewServer(
		fs,
		bridge.A,
		securityAccessories(alarm, contacts, motions, panicBtn)...,
	)
	if err != nil {
		log.Fatal("fail to start server", "error", err)
	}

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

	log.Info("starting server...")
	if err := server.ListenAndServe(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("failed to close server", "err", err)
	}
}

func securityAccessories(
	alarm *accessory.SecuritySystem,
	contacts []*ContactSensor,
	motions []*MotionSensor,
	panicBtn *accessory.Switch,
) []*accessory.A {
	result := []*accessory.A{alarm.A}
	for _, c := range contacts {
		result = append(result, c.A)
	}
	for _, m := range motions {
		result = append(result, m.A)
	}
	result = append(result, panicBtn.A)
	return result
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func alarmUpdateHandler(
	cli clientProvider,
	cfg Config,
) func(value interface{}, request *http.Request) (response interface{}, code int) {
	return func(v interface{}, _ *http.Request) (response interface{}, code int) {
		switch v.(int) {
		case characteristic.SecuritySystemTargetStateStayArm:
			for _, part := range cfg.StayPartitions {
				log.Info("arm stay", "partition", part)
				if err := cli(func(cli *isecnetv2.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			for _, part := range cfg.AwayPartitions {
				log.Info("arm away", "partition", part)
				if err := cli(func(cli *isecnetv2.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm partition 2", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			for _, part := range cfg.NightPartitions {
				log.Info("arm night", "partition", part)
				if err := cli(func(cli *isecnetv2.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm partition 2", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateDisarm:
			log.Info("disarm")
			if err := cli(func(cli *isecnetv2.Client) error {
				return cli.Disarm(isecnetv2.AllPartitions)
			}); err != nil {
				log.Error("could not disarm", "err", err)
				return nil, hap.JsonStatusInvalidValueInRequest
			}
		default:
			return nil, hap.JsonStatusResourceDoesNotExist
		}
		return nil, hap.JsonStatusSuccess
	}
}

func toPartition(i int) byte {
	if i == 0 {
		return isecnetv2.AllPartitions
	}
	return byte(i)
}
