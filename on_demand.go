package renewable

import (
	"context"
	"sync"

	"github.com/SladeThe/renewable/periods"
)

// OnDemand returns a new instance of Renewable that will call produce function only on demand.
//
// First time Renewable.Get is called, produce is called and the result is cached.
// Every next time Get is called, it checks the current time against periods,
// depending on the result of the previous produce call.
// If time is expired, produce is called again, otherwise the cached value is returned.
//
// OnDemand uses read-write synchronization and never performs simultaneous calls of produce.
//
// Neither ctx nor produce can be nil.
// None of the periods can be negative.
// A zero period is a corner case and means, that produce will be called every time Get is called.
func OnDemand(ctx context.Context, produce ProduceFunc, pp periods.Periods) (Renewable, error) {
	if ctx == nil {
		return nil, ErrContextNil
	}

	if produce == nil {
		return nil, ErrProduceNil
	}

	if errValidate := pp.Validate(); errValidate != nil {
		return nil, errValidate
	}

	return &onDemand{
		ctx:     ctx,
		produce: produce,
		periods: pp,
		result:  &result{},
		lock:    &sync.RWMutex{},
	}, nil
}

// OnDemandNoCtx just calls OnDemand(context.Background(), ...).
func OnDemandNoCtx(produce ProduceFunc, pp periods.Periods) (Renewable, error) {
	return OnDemand(context.Background(), produce, pp)
}

type onDemand struct {
	ctx     context.Context
	produce ProduceFunc
	periods periods.Periods
	result  *result

	lock *sync.RWMutex
}

var _ Renewable = (*onDemand)(nil)

func (r *onDemand) get() *result {
	if r.result.isValidNow(r.periods) {
		return r.result
	}

	return nil
}

func (r *onDemand) Get() (interface{}, error) {
	r.lock.RLock()
	res := r.get()
	r.lock.RUnlock()

	if res != nil {
		return res.expand()
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if res = r.get(); res != nil {
		return res.expand()
	}

	r.result.produce(r.ctx, r.produce)
	return r.result.expand()
}
