package renewable

import (
	"context"
	"errors"
	"time"

	"github.com/SladeThe/renewable/periods"
)

var (
	ErrContextNil = errors.New("non-empty context is required")
	ErrProduceNil = errors.New("produce must not be nil")
)

type ProduceFunc func(ctx context.Context) (interface{}, error)

type Renewable interface {
	// Get produces new or returns cached value according to the Renewable strategy.
	// It is always thread safe to call this method.
	Get() (interface{}, error)
}

// Must invokes Renewable.Get and panics in case of an error.
func Must(r Renewable) interface{} {
	if value, err := r.Get(); err != nil {
		panic(err)
	} else {
		return value
	}
}

type result struct {
	val    interface{}
	err    error
	moment time.Time
}

func (r result) isValidAt(moment time.Time, periods periods.Periods) bool {
	return !r.moment.IsZero() && r.moment.Add(periods.Period(r.err)).After(moment)
}

func (r result) isValidNow(periods periods.Periods) bool {
	return r.isValidAt(time.Now(), periods)
}

func (r *result) produce(ctx context.Context, produce ProduceFunc) {
	r.val, r.err = produce(ctx)
	r.moment = time.Now()
}

func (r result) expand() (interface{}, error) {
	return r.val, r.err
}

func produceResult(ctx context.Context, produce ProduceFunc) result {
	var r result
	r.produce(ctx, produce)
	return r
}
