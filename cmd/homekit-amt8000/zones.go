package main

import (
	"net/http"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	client "github.com/caarlos0/homekit-amt8000"
)

const zoneIDStart uint64 = 100

func setupZones(
	cli clientProvider,
	cfg Config,
	status client.Status,
) []*AlarmSensor {
	var sensors []*AlarmSensor
	for _, zone := range cfg.allZones() {
		zone := zone

		bypassFn := func(value interface{}, _ *http.Request) (response interface{}, code int) {
			v := value.(bool)
			log.Info("set zone bypass", "zone", zone.number, "bypass", v)
			if err := cli(func(cli *client.Client) error {
				return cli.Bypass(zone.number, !v)
			}); err != nil {
				log.Error("failed to set bypass", "zone", zone.number, "value", v, "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
			return nil, hap.JsonStatusSuccess
		}

		a := newAlarmSensor(accessory.Info{
			Name:         zone.name,
			Manufacturer: manufacturer,
		}, zone.kind)
		a.Id = zoneIDStart + uint64(zone.number)

		a.Bypass.On.SetValueRequestFunc = bypassFn
		a.Update(status.Zones[zone.number])

		sensors = append(sensors, a)
	}
	return sensors
}
