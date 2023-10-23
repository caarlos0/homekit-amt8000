package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/caarlos0/homekit-amt8000/isecnetv2"
)

type ContactSensors []*ContactSensor

func (contacts ContactSensors) Update(cfg Config, status isecnetv2.Status) {
	for i, zone := range cfg.ContactZones {
		evt := status.Zones[zone-1].AnyEvent()
		current := boolToInt(evt != isecnetv2.ZoneEventClean)
		if v := contacts[i].Contact.ContactSensorState.Value(); v == current {
			continue
		}
		_ = contacts[i].Contact.ContactSensorState.SetValue(current)
		log.Info("contact", "zone", zone, "status", current, "event", evt)
	}
}

type MotionSensors []*MotionSensor

func (motions MotionSensors) Update(cfg Config, status isecnetv2.Status) {
	for i, zone := range cfg.MotionZones {
		evt := status.Zones[zone-1].AnyEvent()
		current := evt != isecnetv2.ZoneEventClean
		if v := motions[i].Motion.MotionDetected.Value(); v == current {
			continue
		}
		motions[i].Motion.MotionDetected.SetValue(current)
		log.Info("motion", "zone", zone, "status", current, "event", evt)
	}
}

type ContactSensor struct {
	*accessory.A
	Contact *service.ContactSensor
	Bypass  *service.Switch
}

func newContactSensor(info accessory.Info) *ContactSensor {
	a := ContactSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Contact = service.NewContactSensor()
	a.AddS(a.Contact.S)

	a.Bypass = service.NewSwitch()
	a.AddS(a.Bypass.S)

	return &a
}

type MotionSensor struct {
	*accessory.A
	Motion *service.MotionSensor
	Bypass *service.Switch
}

func newMotionSensor(info accessory.Info) *MotionSensor {
	a := MotionSensor{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Motion = service.NewMotionSensor()
	a.AddS(a.Motion.S)

	a.Bypass = service.NewSwitch()
	a.AddS(a.Bypass.S)

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
