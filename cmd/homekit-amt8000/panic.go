package main

import (
	"net/http"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	client "github.com/caarlos0/homekit-amt8000"
)

func setupPanicButton(withCli clientProvider) *accessory.Switch {
	a := accessory.NewSwitch(accessory.Info{
		Name:         "Audible Panic",
		Manufacturer: manufacturer,
	})
	a.Switch.On.SetValueRequestFunc = func(value interface{}, _ *http.Request) (response interface{}, code int) {
		v := value.(bool)
		if err := withCli(func(cli *client.Client) error {
			if v {
				log.Warn("triggering an audible panic!")
				return cli.Panic()
			}
			return cli.Disarm(client.AllPartitions)
		}); err != nil {
			log.Error("failed to trigger an audible panic", "err", err)
			return nil, hap.JsonStatusResourceBusy
		}
		return nil, hap.JsonStatusSuccess
	}
	return a
}
