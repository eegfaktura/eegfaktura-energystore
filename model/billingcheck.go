package model

import (
	"errors"
	"time"
)

const (
	billingCheckMaxPeriod = 367 * 24 * time.Hour // a billing period is at most one year
	billingCheckMaxCps    = 10000
)

// billingCheckMaxEnd keeps the period inside the dates the stores can represent.
var billingCheckMaxEnd = time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC).UnixMilli()

// BillingCheckRequest asks for the data checks before a billing run (platform#148): the period
// [Start, End] (epoch ms, the first and the last day of the period) and the metering points active
// in it, each with its own active window (epoch ms of the day, like the Excel export).
type BillingCheckRequest struct {
	Start int64            `json:"start"`
	End   int64            `json:"end"`
	Cps   []BillingCheckCp `json:"cps"`
}

type BillingCheckCp struct {
	MeteringPoint string         `json:"meteringPoint"`
	Direction     MeterDirection `json:"direction"`
	ActiveSince   int64          `json:"activeSince"`
	InactiveSince int64          `json:"inactiveSince"`
}

// BillingCheckMeter is the result for one requested entry, in request order. A metering point
// listed twice (owner change) gets one entry per active window.
type BillingCheckMeter struct {
	MeteringPoint string         `json:"meteringPoint"`
	Direction     MeterDirection `json:"direction"`
	ActiveSince   int64          `json:"activeSince"`
	InactiveSince int64          `json:"inactiveSince"`
	// Quarter hours in the active window with at least one L3 value of this metering point,
	// and the days (YYYY-MM-DD, chronological) they fall on.
	L3Count int      `json:"l3Count"`
	L3Days  []string `json:"l3Days"`
	// First and last quarter hour (epoch ms) in the active window with a value (quality not L0);
	// nil if there is none.
	FirstData *int64 `json:"firstData"`
	LastData  *int64 `json:"lastData"`
	// Quarter hours without a value in the active part of the period (also before the first and
	// after the last value), and the days (YYYY-MM-DD, chronological) with at least one of them.
	// A day has its quarter hours in local time (92 on the spring clock change day).
	MissingCount int      `json:"missingCount"`
	MissingDays  []string `json:"missingDays"`
}

// BillingCheckTotals sums the values of all metering points of the community in the period.
type BillingCheckTotals struct {
	Produced    float64 `json:"produced"`    // producer G.01 (production)
	Consumed    float64 `json:"consumed"`    // consumer G.01 (consumption)
	Allocated   float64 `json:"allocated"`   // consumer G.02 (share of the community production)
	SelfCovered float64 `json:"selfCovered"` // consumer G.03 (self coverage)
	Surplus     float64 `json:"surplus"`     // producer P.01 (surplus)
}

type BillingCheckResponse struct {
	// First and last quarter hour of the period (epoch ms).
	PeriodStart int64               `json:"periodStart"`
	PeriodEnd   int64               `json:"periodEnd"`
	Meters      []BillingCheckMeter `json:"meters"`
	Totals      BillingCheckTotals  `json:"totals"`
}

// Validate rejects requests that cannot be a billing period of one community.
func (req *BillingCheckRequest) Validate() error {
	if req.Start <= 0 || req.End < req.Start || req.End > billingCheckMaxEnd {
		return errors.New("invalid period")
	}
	if req.End-req.Start > billingCheckMaxPeriod.Milliseconds() {
		return errors.New("period longer than one year")
	}
	if len(req.Cps) > billingCheckMaxCps {
		return errors.New("too many metering points")
	}
	for _, cp := range req.Cps {
		if cp.MeteringPoint == "" {
			return errors.New("metering point missing")
		}
		if cp.Direction != CONSUMER_DIRECTION && cp.Direction != PRODUCER_DIRECTION {
			return errors.New("invalid direction")
		}
	}
	return nil
}
