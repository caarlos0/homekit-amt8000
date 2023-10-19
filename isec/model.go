package isec

type OverallStatus struct {
	Model       string
	Version     string
	Status      State
	ZonesFiring bool
	ZonesClosed bool
	Siren       bool
	Partitions  []Partition
	Zones       []Zone
}

type Zone struct {
	Number       int
	Open         bool
	Violated     bool
	Anulated     bool
	LowBattery   bool
	Tamper       bool
	ShortCircuit bool
}

type Partition struct {
	Number int
	Armed  bool
	Fired  bool
	Firing bool
	Stay   bool
}
