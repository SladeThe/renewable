package renewable

import (
	"testing"

	"github.com/SladeThe/renewable/periods"
)

func TestSoftHard_Get(t *testing.T) {
	simpleTestRenewable(t, func(produce ProduceFunc) (Renewable, error) {
		pp := periods.New(defaultPeriod, errorPeriod)
		return SoftHardNoCtx(produce, pp, pp)
	})
}

func TestSoftHard_Get_Async(t *testing.T) {
	asyncTestRenewable(t, func(produce ProduceFunc) (Renewable, error) {
		pp := periods.New(defaultPeriod, errorPeriod)
		return SoftHardNoCtx(produce, pp, pp)
	}, asyncTestRenewableOnce)
}

func TestSoftHard_Get_AsyncSoftHard(t *testing.T) {
	asyncTestRenewable(t, func(produce ProduceFunc) (Renewable, error) {
		return SoftHardNoCtx(
			produce,
			periods.New(defaultPeriod, errorPeriod),
			periods.New(defaultPeriod*2, errorPeriod*2),
		)
	}, asyncTestRenewableOnce)
}
