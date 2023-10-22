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

	log.Info(
		"loading accessories",
		"configuration", strings.Join([]string{
			fmt.Sprintf("stay partition %d", cfg.StayPartition),
			fmt.Sprintf("away_partition %d", cfg.AwayPartition),
			fmt.Sprintf("night_partition %d", cfg.NightPartition),
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
	if state := toCurrentState(cfg, status); state >= 0 {
		err := alarm.SecuritySystem.SecuritySystemTargetState.SetValue(state)
		log.Info("set target state", "state", state, "err", err)
	}
	alarm.SecuritySystem.SecuritySystemTargetState.SetValueRequestFunc = alarmUpdateHandler(
		withCli,
		cfg,
	)

	contacts, motions, bypasses := setupZones(withCli, cfg, status)

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

			if state := toCurrentState(cfg, status); alarm.SecuritySystem.SecuritySystemCurrentState.Value() != state {
				err := alarm.SecuritySystem.SecuritySystemCurrentState.SetValue(state)
				log.Info("set current state", "state", state, "err", err)
			}

			BypassSwitches(bypasses).Update(cfg, status)
			ContactSensors(contacts).Update(cfg, status)
			MotionSensors(motions).Update(cfg, status)
			panicBtn.Switch.On.SetValue(status.Siren)
		}
	}()

	fs := hap.NewFsStore("./db")

	server, err := hap.NewServer(
		fs,
		bridge.A,
		securityAccessories(alarm, contacts, motions, bypasses, panicBtn)...,
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

func toCurrentState(cfg Config, status isecnetv2.Status) int {
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
	panicBtn *accessory.Switch,
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
			log.Info("arm stay")
			if err := cli(func(cli *isecnetv2.Client) error {
				return cli.Arm(toPartition(cfg.StayPartition))
			}); err != nil {
				log.Error("could not arm", "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			log.Info("arm away")
			if err := cli(func(cli *isecnetv2.Client) error {
				return cli.Arm(toPartition(cfg.AwayPartition))
			}); err != nil {
				log.Error("could not arm partition 2", "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			log.Info("arm night")
			if err := cli(func(cli *isecnetv2.Client) error {
				return cli.Arm(toPartition(cfg.NightPartition))
			}); err != nil {
				log.Error("could not arm partition 2", "err", err)
				return nil, hap.JsonStatusResourceBusy
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
