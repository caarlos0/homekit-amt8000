package main

import (
	"fmt"
	"net/http"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
)

func setupZones(
	cli *isecnetv2.Client,
	cfg Config,
	status isecnetv2.Status,
) ([]*ContactSensor, []*MotionSensor, []*accessory.Switch) {
	return setupContactZones(cfg, status),
		setupMotionZones(cfg, status),
		setupBypassZones(cli, cfg, status)
}

func zoneName(cfg Config, n int, s string) string {
	names := cfg.ZoneNames
	if len(names) > n-1 {
		return fmt.Sprintf("%s %s", names[n-1], s)
	}
	return fmt.Sprintf("Zone %d %s", n, s)
}

func setupMotionZones(
	cfg Config,
	status isecnetv2.Status,
) []*MotionSensor {
	motions := make([]*MotionSensor, len(cfg.MotionZones))
	for i, zone := range cfg.MotionZones {
		a := newMotionSensor(accessory.Info{
			Name:         zoneName(cfg, zone, "sensor"),
			Manufacturer: manufacturer,
		})
		if status.Zones[zone-1].Open {
			a.MotionSensor.MotionDetected.SetValue(true)
		}
		motions[i] = a
	}
	return motions
}

func setupContactZones(
	cfg Config,
	status isecnetv2.Status,
) []*ContactSensor {
	contacts := make([]*ContactSensor, len(cfg.ContactZones))
	for i, zone := range cfg.ContactZones {
		a := newContactSensor(accessory.Info{
			Name:         zoneName(cfg, zone, "sensor"),
			Manufacturer: manufacturer,
		})
		if status.Zones[zone-1].Open {
			_ = a.ContactSensor.ContactSensorState.SetValue(1)
		}
		contacts[i] = a
	}

	return contacts
}

func setupBypassZones(
	cli *isecnetv2.Client,
	cfg Config,
	status isecnetv2.Status,
) []*accessory.Switch {
	bypasses := make([]*accessory.Switch, len(cfg.AllowBypassZones))
	for i, zone := range cfg.AllowBypassZones {
		zone := zone
		a := accessory.NewSwitch(accessory.Info{
			Name:         zoneName(cfg, zone, "bypass"),
			Manufacturer: manufacturer,
		})
		a.Switch.On.SetValueRequestFunc = func(value interface{}, _ *http.Request) (response interface{}, code int) {
			clientLock.Lock()
			defer clientLock.Unlock()
			v := value.(bool)
			log.Info("set zone bypass", "zone", zone, "bypass", v)
			if err := cli.Bypass(zone, v); err != nil {
				log.Error("failed to set bypass", "zone", zone, "value", v, "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
			return nil, hap.JsonStatusSuccess
		}
		if status.Zones[zone-1].Anulated {
			a.Switch.On.SetValue(true)
		}
		bypasses[i] = a
	}
	return bypasses
}
