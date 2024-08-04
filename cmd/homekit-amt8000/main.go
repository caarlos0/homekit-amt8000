package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/caarlos0/env/v11"
	client "github.com/caarlos0/homekit-amt8000"
	"github.com/cenkalti/backoff/v4"
	logp "github.com/charmbracelet/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed index.html
var index []byte

var log = logp.NewWithOptions(os.Stderr, logp.Options{
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
	Prefix:          "homekit",
})

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Executor = func(func(cli *client.Client) error) error

const (
	manufacturer = "Intelbras"
	retries      = 5
)

func main() {
	log.Info(
		"homekit-amt8000",
		"version", version,
		"commit", commit,
		"date", date,
		"info", strings.Join([]string{
			"Homekit bridge for Intelbras AMT8000 alarm systems",
			"© Carlos Alexandro Becker",
			"https://becker.software",
		}, "\n"),
	)

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(
			"could not parse env",
			"err",
			strings.TrimPrefix(strings.ReplaceAll(err.Error(), "; ", "\n"), "env: ")+"\n",
		)
	}

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
			requestCounter.Inc()
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
				requestErrorCounter.Inc()
				if errors.Is(err, client.ErrOpenZones) ||
					errors.Is(err, client.ErrInvalidPassword) {
					return backoff.Permanent(err)
				}
				return err
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
			"could not get the mac address, needs 'cap_net_raw+ep' capabilities",
			"err", err,
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
		Firmware:     version,
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

			if len(status.Zones) >= len(cfg.allZones()) {
				for i, zi := range cfg.allZones() {
					zone := status.Zones[zi.number-1]
					sensor := sensors[i]
					sensor.Update(zone)
				}
			}
			if len(status.Sirens) >= len(cfg.Sirens) {
				for i, number := range cfg.Sirens {
					sirens[i].Update(status.Sirens[number-1])
				}
			}
			if len(status.Repeaters) >= len(cfg.Repeaters) {
				for i, number := range cfg.Repeaters {
					repeaters[i].Update(status.Repeaters[number-1])
				}
			}
		}
	}()

	fs := hap.NewFsStore("./db")

	server, err := hap.NewServer(
		fs, bridge.A,
		securityAccessories(sensors, sirens, repeaters, alarm, panicBtn)...,
	)
	if err != nil {
		log.Fatal("fail to create server", "error", err)
	}
	server.Addr = cfg.Address
	server.ServeMux().Handle("/metrics", promhttp.Handler())
	server.ServeMux().Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := [5]string{
			"Armed: Stay",
			"Armed: Away",
			"Armed: Night",
			"Disarmed",
			"Alarm Triggered",
		}[alarm.SecuritySystem.SecuritySystemCurrentState.Value()]

		var hSensors []PageItem
		for i, zone := range sensors {
			z := PageItem{
				Number:     i + 1,
				Name:       zone.Name(),
				Tamper:     zone.Tamper.Value() == 1,
				LowBattery: zone.LowBattery.Value() == 1,
			}
			if zone.Motion != nil {
				z.Open = zone.Motion.MotionDetected.Value()
			} else if zone.Contact != nil {
				z.Open = zone.Contact.ContactSensorState.Value() == 1
			}
			if zone.Bypass != nil {
				z.Bypassed = zone.Bypass.On.Value()
			}
			hSensors = append(hSensors, z)
		}

		var hSirens []PageItem
		for i, siren := range sirens {
			hSirens = append(hSirens, PageItem{
				Number:     i + 1,
				Name:       siren.Name(),
				Tamper:     siren.Tamper.Value() == 1,
				LowBattery: siren.LowBattery.Value() == 1,
			})
		}

		var hRepeaters []PageItem
		for i, repeater := range repeaters {
			hRepeaters = append(hRepeaters, PageItem{
				Number:     i + 1,
				Name:       repeater.Name(),
				Tamper:     repeater.Tamper.Value() == 1,
				LowBattery: repeater.LowBattery.Value() == 1,
			})
		}

		tpl := template.Must(template.New("index").Parse(string(index)))
		_ = tpl.Execute(w, struct {
			State     string
			Zones     []PageItem
			Sirens    []PageItem
			Repeaters []PageItem
		}{
			State:     state,
			Zones:     hSensors,
			Sirens:    hSirens,
			Repeaters: hRepeaters,
		})
	}))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		log.Info("stopping server")
		signal.Stop(c)
		cancel()
	}()

	log.Info("starting server", "addr", server.Addr)
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

func boolAs[T int | float64](b bool) T {
	if b {
		return 1
	}
	return 0
}

type PageItem struct {
	Number     int
	Name       string
	Open       bool
	Tamper     bool
	Bypassed   bool
	LowBattery bool
}
