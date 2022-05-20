package renewable

import (
	"testing"

	"github.com/SladeThe/renewable/periods"
)

func TestOnDemand_Get(t *testing.T) {
	simpleTestRenewable(t, func(produce ProduceFunc) (Renewable, error) {
		return OnDemandNoCtx(produce, periods.New(defaultPeriod, errorPeriod))
	})
}

func TestOnDemand_Get_Async(t *testing.T) {
	asyncTestRenewable(t, func(produce ProduceFunc) (Renewable, error) {
		return OnDemandNoCtx(produce, periods.New(defaultPeriod, errorPeriod))
	}, asyncTestRenewableOnce)
}
