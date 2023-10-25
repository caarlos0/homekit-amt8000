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
	"github.com/caarlos0/env/v9"
	goversion "github.com/caarlos0/go-version"
	client "github.com/caarlos0/homekit-amt8000"
	"github.com/cenkalti/backoff/v4"
	logp "github.com/charmbracelet/log"
)

var log = logp.NewWithOptions(os.Stderr, logp.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
	Prefix:          "homekit",
})

type Executor = func(func(cli *client.Client) error) error

const (
	manufacturer = "Intelbras"
	retries      = 5
)

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("could not parse env", "err", err)
	}

	info := goversion.GetVersionInfo(
		goversion.WithAppDetails(
			"homekit-amt8000",
			"Unofficial Homekit bridge for the Intelbras AMT8000 alarm system",
			"© Carlos A. Becker - https://becker.software",
		),
	)
	fmt.Println(info.String())

	log.Info(
		"loading accessories",
		"partitions",
		strings.Join([]string{
			fmt.Sprintf("stay: %v", cfg.StayPartitions),
			fmt.Sprintf("away: %v", cfg.AwayPartitions),
			fmt.Sprintf("night: %v", cfg.NightPartitions),
		}, "\n"),
		"zones", allZoneConfigs(cfg.allZones()).String(),
	)

	var clientLock sync.Mutex
	execute := func(fn func(cli *client.Client) error) error {
		t := time.Now()
		clientLock.Lock()
		defer clientLock.Unlock()
		log.Debugf("got client lock after %s", time.Since(t))

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Second * 5
		bo.MaxElapsedTime = time.Minute

		return backoff.RetryNotify(func() error {
			cli, err := client.New(cfg.Host, cfg.Port, cfg.Password)
			if err != nil {
				return fmt.Errorf("could not init isecnet2 client: %w", err)
			}
			defer func() {
				if err := cli.Close(); err != nil {
					log.Error("could not close isecnet2 client", "err", err)
				}
			}()
			if err := fn(cli); err != nil {
				if errors.Is(err, client.ErrOpenZones) ||
					errors.Is(err, client.ErrInvalidPassword) {
					return backoff.Permanent(err)
				}
			}
			return nil
		}, bo, func(err error, _ time.Duration) {
			log.Error("command to central failed", "err", err)
		})
	}

	var status client.Status
	if err := execute(func(cli *client.Client) (err error) {
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
		"got alarm system information",
		"manufacturer", manufacturer,
		"model", status.Model,
		"version", status.Version,
		"mac", macAddr,
	)

	bridge := accessory.NewBridge(accessory.Info{
		Name:         "Alarm Bridge",
		Manufacturer: manufacturer,
		Firmware:     info.GitVersion,
	})

	alarm := NewSecuritySystem(accessory.Info{
		Name:         "Alarm",
		SerialNumber: macAddr,
		Manufacturer: manufacturer,
		Model:        status.Model,
		Firmware:     status.Version,
	}, cfg, execute)
	alarm.Id = 2

	if state := cfg.getAlarmState(status); state >= 0 {
		err := alarm.SecuritySystem.SecuritySystemTargetState.SetValue(state)
		log.Info("set target state", "state", state, "err", err)
	}

	panicBtn := setupPanicButton(execute)
	panicBtn.Id = 3

	sensors := setupZones(execute, cfg, status)
	sirens := setupSirens(cfg, status)
	repeaters := setupRepeaters(cfg, status)

	go func() {
		tick := time.NewTicker(time.Second * 3)
		for range tick.C {
			var status client.Status
			if err := execute(func(cli *client.Client) (err error) {
				status, err = cli.Status()
				return
			}); err != nil {
				log.Error("could not get status", "err", err)
				continue
			}

			alarm.Update(status)
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

func toPartition(i int) byte {
	if i == 0 {
		return client.AllPartitions
	}
	return byte(i)
}
