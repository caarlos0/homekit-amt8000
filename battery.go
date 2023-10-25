package amt8000

type BatteryStatus uint8

const (
	BatteryStatusUnknown BatteryStatus = iota
	BatteryStatusMissing
	BatteryStatusShortCircuited
	BatteryStatusDead
	BatteryStatusLow
	BatteryStatusMiddle
	BatteryStatusFull
)

func (b BatteryStatus) String() string {
	switch b {
	case BatteryStatusMissing:
		return "missing"
	case BatteryStatusShortCircuited:
		return "short-circuited"
	case BatteryStatusDead:
		return "dead"
	case BatteryStatusLow:
		return "low"
	case BatteryStatusMiddle:
		return "middle"
	case BatteryStatusFull:
		return "full"
	default:
		return "unknown"
	}
}

func (b BatteryStatus) Level() int {
	switch b {
	case BatteryStatusLow:
		return 20
	case BatteryStatusMiddle:
		return 50
	case BatteryStatusFull:
		return 100
	default: // <= Dead
		return 0
	}
}

func batteryStatusFor(resp []byte) BatteryStatus {
	generalTroubles := resp[71]
	switch {
	case generalTroubles&(1<<0x04) > 0:
		return BatteryStatusShortCircuited
	case generalTroubles&(1<<0x05) > 0:
		return BatteryStatusMissing
	}

	batt := resp[134]
	switch {
	case batt&0x01 == 0x01:
		return BatteryStatusDead
	case batt&0x02 == 0x02:
		return BatteryStatusLow
	case batt&0x03 == 0x03:
		return BatteryStatusMiddle
	case batt&0x04 == 0x04:
		return BatteryStatusFull
	default:
		return BatteryStatusUnknown
	}
}
