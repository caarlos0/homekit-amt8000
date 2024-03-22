package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var armStateGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "alarm",
	Name:        "state",
	Help:        "",
	ConstLabels: map[string]string{},
})

var tamperGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "alarm",
	Name:        "tamper",
	Help:        "",
	ConstLabels: map[string]string{},
}, []string{"name"})

var openGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "alarm",
	Name:        "open",
	Help:        "",
	ConstLabels: map[string]string{},
}, []string{"name"})

var violatedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "alarm",
	Name:        "violated",
	Help:        "",
	ConstLabels: map[string]string{},
}, []string{"name"})

var bypassedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "alarm",
	Name:        "bypassed",
	Help:        "",
	ConstLabels: map[string]string{},
}, []string{"name"})

var requestCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "client",
	Name:        "requests_total",
	Help:        "",
	ConstLabels: map[string]string{},
})

var requestErrorCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace:   "homekit_amt8000",
	Subsystem:   "client",
	Name:        "request_errors_total",
	Help:        "",
	ConstLabels: map[string]string{},
})
