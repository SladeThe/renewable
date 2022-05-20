package renewable

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SladeThe/renewable/periods"
)

// SoftHard returns a new instance of Renewable that will use the soft-hard periods strategy.
//
// First time Renewable.Get is called, produce is called and the result is cached.
// Every next time Get is called, it checks the current time against periods,
// depending on the result of the previous produce call.
//
// If soft deadline is NOT passed, the cached value is returned.
// Otherwise, if hard deadline is NOT passed, the cached value is returned
// and a goroutine is started to update the result asynchronously.
// If both periods are expired, produce is called again and the caller waits for it to complete.
//
// This renewable doesn't perform simultaneous calls of produce.
//
// Neither ctx nor produce can be nil.
// None of the periods can be negative.
// None of the hard periods can be less, than corresponding soft period.
// A zero period is a corner case and means, that produce will be called every time Get is called.
func SoftHard(ctx context.Context, produce ProduceFunc, soft, hard periods.Periods) (Renewable, error) {
	if ctx == nil {
		return nil, ErrContextNil
	}

	if produce == nil {
		return nil, ErrProduceNil
	}

	if errValidate := soft.Validate(); errValidate != nil {
		return nil, errValidate
	}

	if hard.Default < soft.Default {
		return nil, fmt.Errorf(
			"default hard period must be equal or greater than soft, but got %v < %v",
			hard.Default, soft.Default,
		)
	}

	if hard.Error < soft.Error {
		return nil, fmt.Errorf(
			"error hard period must be equal or greater than soft, but got %v < %v",
			hard.Error, soft.Error,
		)
	}

	mutex := &sync.Mutex{}

	return &softHard{
		ctx:       ctx,
		produce:   produce,
		soft:      soft,
		hard:      hard,
		result:    &atomic.Value{},
		lock:      mutex,
		condition: sync.NewCond(mutex),
	}, nil
}

func SoftHardNoCtx(produce ProduceFunc, soft, hard periods.Periods) (Renewable, error) {
	return SoftHard(context.Background(), produce, soft, hard)
}

var _ Renewable = (*softHard)(nil)

type softHard struct {
	ctx     context.Context
	produce ProduceFunc
	soft    periods.Periods
	hard    periods.Periods
	result  *atomic.Value

	lock      *sync.Mutex
	condition *sync.Cond
	state     uint32
}

func (r *softHard) updateAsync() {
	go func() {
		r.result.Store(produceResult(r.ctx, r.produce))

		r.lock.Lock()
		defer r.lock.Unlock()

		atomic.StoreUint32(&r.state, 0)
		r.condition.Broadcast()
	}()
}

func (r *softHard) Get() (interface{}, error) {
	if res, ok := r.result.Load().(result); ok {
		now := time.Now()

		if res.isValidAt(now, r.soft) {
			return res.expand()
		}

		if res.isValidAt(now, r.hard) {
			if atomic.CompareAndSwapUint32(&r.state, 0, 1) {
				if res = r.result.Load().(result); res.isValidAt(now, r.soft) {
					r.lock.Lock()
					defer r.lock.Unlock()

					atomic.StoreUint32(&r.state, 0)
					r.condition.Broadcast()
				} else {
					r.updateAsync()
				}
			}

			return res.expand()
		}
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if res, ok := r.result.Load().(result); ok && res.isValidNow(r.soft) {
		return res.expand()
	}

	for !atomic.CompareAndSwapUint32(&r.state, 0, 1) {
		for atomic.LoadUint32(&r.state) > 0 {
			r.condition.Wait()
		}
	}

	if res, ok := r.result.Load().(result); ok {
		now := time.Now()

		if res.isValidAt(now, r.soft) {
			atomic.StoreUint32(&r.state, 0)
			r.condition.Broadcast()
			return res.expand()
		}

		if res.isValidAt(now, r.hard) {
			r.updateAsync()
			return res.expand()
		}
	}

	res := produceResult(r.ctx, r.produce)
	r.result.Store(res)

	atomic.StoreUint32(&r.state, 0)
	r.condition.Broadcast()

	return res.expand()
}
