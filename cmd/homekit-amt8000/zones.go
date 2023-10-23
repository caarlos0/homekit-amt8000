package main

import (
	"net/http"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	client "github.com/caarlos0/homekit-amt8000"
)

func setupZones(
	cli clientProvider,
	cfg Config,
	status client.Status,
) ([]*ContactSensor, []*MotionSensor) {
	var contacts []*ContactSensor
	var motions []*MotionSensor
	for _, zone := range cfg.allZones() {
		zone := zone

		bypassing := status.Zones[zone.number].Anulated
		bypassFn := func(value interface{}, _ *http.Request) (response interface{}, code int) {
			v := value.(bool)
			log.Info("set zone bypass", "zone", zone, "bypass", v)
			if err := cli(func(cli *client.Client) error {
				return cli.Bypass(zone.number, !v)
			}); err != nil {
				log.Error("failed to set bypass", "zone", zone, "value", v, "err", err)
				return nil, hap.JsonStatusResourceBusy
			}
			return nil, hap.JsonStatusSuccess
		}

		switch zone.kind {
		case kindMotion:
			a := newMotionSensor(accessory.Info{
				Name:         zone.name,
				Manufacturer: manufacturer,
			})
			if status.Zones[zone.number].IsOpen() {
				a.Motion.MotionDetected.SetValue(true)
			}

			a.Bypass.On.SetValue(!bypassing)
			a.Bypass.On.SetValueRequestFunc = bypassFn
			motions = append(motions, a)
		case kindContact:
			a := newContactSensor(accessory.Info{
				Name:         zone.name,
				Manufacturer: manufacturer,
			})
			if status.Zones[zone.number].IsOpen() {
				_ = a.Contact.ContactSensorState.SetValue(1)
			}
			a.Bypass.On.SetValue(!bypassing)
			a.Bypass.On.SetValueRequestFunc = bypassFn
			contacts = append(contacts, a)
		}
	}
	return contacts, motions
}
