package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v9"
	client "github.com/caarlos0/homekit-amt8000"
	"github.com/cenkalti/backoff/v4"
	logp "github.com/charmbracelet/log"
	str "github.com/charmbracelet/x/exp/strings"
)

var log = logp.NewWithOptions(os.Stderr, logp.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
	Prefix:          "homekit",
})

type clientProvider = func(func(cli *client.Client) error) error

const (
	manufacturer = "Intelbras"
	retries      = 5
)

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("could not parse env", "err", err)
	}

	log.Info(
		"loading accessories",
		"partitions",
		strings.Join([]string{
			fmt.Sprintf("stay: %s", intJoin(cfg.StayPartitions)),
			fmt.Sprintf("away: %s", intJoin(cfg.AwayPartitions)),
			fmt.Sprintf("night: %s", intJoin(cfg.NightPartitions)),
		}, "\n"),
		"zones", allZoneConfigs(cfg.allZones()).String(),
	)

	var clientLock sync.Mutex
	withCli := func(fn func(cli *client.Client) error) error {
		t := time.Now()
		clientLock.Lock()
		defer clientLock.Unlock()
		log.Debugf("got client lock after %s", time.Since(t))

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Second * 5
		bo.MaxElapsedTime = time.Minute

		return backoff.Retry(func() error {
			cli, err := client.New(cfg.Host, cfg.Port, cfg.Password)
			if err != nil {
				return fmt.Errorf("could not init isecnet2 client: %w", err)
			}
			defer func() {
				if err := cli.Close(); err != nil {
					log.Error("could not close isecnet2 client", "err", err)
				}
			}()
			return fn(cli)
		}, bo)
	}

	var status client.Status
	if err := withCli(func(cli *client.Client) (err error) {
		status, err = cli.Status()
		return
	}); err != nil {
		log.Fatal("could not init accessories", "err", err)
	}
	macAddr, err := client.MacAddress(cfg.Host)
	if err != nil {
		log.Warn(
			"could not get the mac address, maybe run with 'sudo setcap cap_net_raw+ep'?",
			"err",
			err,
		)
	}
	log.Info(
		"got system information",
		"manufacturer", manufacturer,
		"model", status.Model,
		"version", status.Version,
		"mac", macAddr,
	)

	bridge := accessory.NewBridge(accessory.Info{
		Name:         "Alarm Bridge",
		Manufacturer: manufacturer,
	})

	alarm := NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		SerialNumber: macAddr,
		Manufacturer: manufacturer,
		Model:        status.Model,
		Firmware:     status.Version,
	})
	alarm.Id = 2

	if state := cfg.getAlarmState(status); state >= 0 {
		err := alarm.SecuritySystem.SecuritySystemTargetState.SetValue(state)
		log.Info("set target state", "state", state, "err", err)
	}
	alarm.SecuritySystem.SecuritySystemTargetState.SetValueRequestFunc = alarmUpdateHandler(
		withCli,
		cfg,
	)

	panicBtn := accessory.NewSwitch(accessory.Info{
		Name:         "Trigger panic",
		Manufacturer: manufacturer,
	})
	panicBtn.Id = 3
	panicBtn.Switch.On.SetValueRequestFunc = func(value interface{}, _ *http.Request) (response interface{}, code int) {
		v := value.(bool)
		if err := withCli(func(cli *client.Client) error {
			if v {
				log.Warn("triggering a panic!")
				return cli.Panic()
			}
			return cli.Disarm(client.AllPartitions)
		}); err != nil {
			log.Error("failed to trigger panic", "err", err)
			return nil, hap.JsonStatusResourceBusy
		}
		return nil, hap.JsonStatusSuccess
	}

	sensors := setupZones(withCli, cfg, status)
	sirens := setupSirens(cfg, status)
	repeaters := setupRepeaters(cfg, status)

	go func() {
		tick := time.NewTicker(time.Second * 3)
		for range tick.C {
			var status client.Status
			if err := withCli(func(cli *client.Client) (err error) {
				status, err = cli.Status()
				return
			}); err != nil {
				log.Error("could not get status", "err", err)
				continue
			}

			alarm.Update(cfg, status)
			panicBtn.Switch.On.SetValue(status.Siren)

			for i, zi := range cfg.allZones() {
				zone := status.Zones[zi.number-1]
				sensor := sensors[i]
				sensor.Update(zone)
			}
			for i, number := range cfg.Sirens {
				sirens[i].Update(status.Sirens[number-1])
			}
			for i, number := range cfg.Repeaters {
				repeaters[i].Update(status.Repeaters[number-1])
			}
		}
	}()

	fs := hap.NewFsStore("./db")

	server, err := hap.NewServer(
		fs,
		bridge.A,
		securityAccessories(sensors, sirens, repeaters, alarm, panicBtn)...,
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
	sensors []*AlarmSensor,
	sirens []*Siren,
	repeaters []*Repeater,
	alarm *SecuritySystem,
	panicBtn *accessory.Switch,
) []*accessory.A {
	result := []*accessory.A{
		panicBtn.A,
		alarm.A,
	}
	for _, c := range sensors {
		result = append(result, c.A)
	}
	for _, c := range sirens {
		result = append(result, c.A)
	}
	for _, c := range repeaters {
		result = append(result, c.A)
	}
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
				if err := cli(func(cli *client.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateAwayArm:
			for _, part := range cfg.AwayPartitions {
				log.Info("arm away", "partition", part)
				if err := cli(func(cli *client.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm partition 2", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateNightArm:
			for _, part := range cfg.NightPartitions {
				log.Info("arm night", "partition", part)
				if err := cli(func(cli *client.Client) error {
					return cli.Arm(toPartition(part))
				}); err != nil {
					log.Error("could not arm partition 2", "err", err)
					return nil, hap.JsonStatusResourceBusy
				}
			}
		case characteristic.SecuritySystemTargetStateDisarm:
			log.Info("disarm")
			if err := cli(func(cli *client.Client) error {
				return cli.Disarm(client.AllPartitions)
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
		return client.AllPartitions
	}
	return byte(i)
}

func intJoin(zz []int) string {
	zs := make([]string, len(zz))
	for i := range zz {
		zs = append(zs, strconv.Itoa(zz[i]))
	}
	return str.EnglishJoin(zs, true)
}
