package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
)

type BypassSwitches []*accessory.Switch

func (bypasses BypassSwitches) Update(cfg Config, status isecnetv2.Status) {
	for i, zone := range cfg.AllowBypassZones {
		current := status.Zones[zone-1].Anulated
		if v := bypasses[i].Switch.On.Value(); v == current {
			continue
		}
		bypasses[i].Switch.On.SetValue(current)
		log.Info("bypass", "zone", zone, "status", current)
	}
}

type ContactSensors []*ContactSensor

func (contacts ContactSensors) Update(cfg Config, status isecnetv2.Status) {
	for i, zone := range cfg.ContactZones {
		evt := status.Zones[zone-1].AnyEvent()
		current := boolToInt(evt != isecnetv2.ZoneEventClean)
		if v := contacts[i].ContactSensor.ContactSensorState.Value(); v == current {
			continue
		}
		_ = contacts[i].ContactSensor.ContactSensorState.SetValue(current)
		log.Info("contact", "zone", zone, "status", current, "event", evt)
	}
}

type MotionSensors []*MotionSensor

func (motions MotionSensors) Update(cfg Config, status isecnetv2.Status) {
	for i, zone := range cfg.MotionZones {
		evt := status.Zones[zone-1].AnyEvent()
		current := evt != isecnetv2.ZoneEventClean
		if v := motions[i].MotionSensor.MotionDetected.Value(); v == current {
			continue
		}
		motions[i].MotionSensor.MotionDetected.SetValue(current)
		log.Info("motion", "zone", zone, "status", current, "event", evt)
	}
}

type ContactSensor struct {
	*accessory.A
	ContactSensor *service.ContactSensor
}

func newContactSensor(info accessory.Info) *ContactSensor {
	a := ContactSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.ContactSensor = service.NewContactSensor()
	a.AddS(a.ContactSensor.S)

	return &a
}

type MotionSensor struct {
	*accessory.A
	MotionSensor *service.MotionSensor
}

func newMotionSensor(info accessory.Info) *MotionSensor {
	a := MotionSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.MotionSensor = service.NewMotionSensor()
	a.AddS(a.MotionSensor.S)

	return &a
}

type PanicButton struct {
	*accessory.A
	Switch *service.Switch
}

func NewPanicButoon(info accessory.Info) *PanicButton {
	a := PanicButton{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Switch = service.NewSwitch()
	a.AddS(a.Switch.S)

	return &a
}
