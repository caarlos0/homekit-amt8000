package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
)

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
