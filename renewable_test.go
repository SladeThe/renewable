package renewable

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultPeriod   = 75 * time.Millisecond
	errorPeriod     = 100 * time.Millisecond
	safeCheckPeriod = 25 * time.Millisecond
)

// Various const limitations.
//
//noinspection GoBoolExpressions
var (
	_ = map[bool]struct{}{false: {}, safeCheckPeriod > 0: {}}
	_ = map[bool]struct{}{false: {}, defaultPeriod >= safeCheckPeriod*3: {}}
	_ = map[bool]struct{}{false: {}, errorPeriod >= safeCheckPeriod*3: {}}

	_ = map[bool]struct{}{
		defaultPeriod > errorPeriod && defaultPeriod-errorPeriod >= safeCheckPeriod: {},
		errorPeriod > defaultPeriod && errorPeriod-defaultPeriod >= safeCheckPeriod: {},
	}
)

func simpleTestRenewable(t *testing.T, createRenewable func(produce ProduceFunc) (Renewable, error)) {
	const iterCount = 10

	results := make([]result, 0, iterCount*2)
	for i := 0; i < iterCount; i++ {
		results = append(results, result{val: i}, result{err: fmt.Errorf("%d", i)})
	}

	renewable, errCreate := createRenewable(func(context.Context) (interface{}, error) {
		if len(results) <= 0 {
			assert.FailNow(t, "results are unexpectedly empty")
		}

		r := results[0]
		results = results[1:]
		return r.val, r.err
	})

	require.NoError(t, errCreate, "failed creating renewable")

	time.Sleep(safeCheckPeriod)
	assert.Equal(t, iterCount*2, len(results))

	for i := 0; i < iterCount; i++ {
		value, err := renewable.Get()
		assert.Equal(t, i, value)
		assert.Nil(t, err)
		assert.Equal(t, iterCount*2-i*2-1, len(results))

		if i > 0 {
			time.Sleep(defaultPeriod - 2*safeCheckPeriod)
		} else {
			time.Sleep(defaultPeriod - safeCheckPeriod)
		}

		value, err = renewable.Get()
		assert.Equal(t, i, value)
		assert.Nil(t, err)
		assert.Equal(t, iterCount*2-i*2-1, len(results))

		time.Sleep(2 * safeCheckPeriod)

		value, err = renewable.Get()
		assert.Nil(t, value)
		assert.Equal(t, fmt.Errorf("%d", i), err)
		assert.Equal(t, iterCount*2-i*2-2, len(results))

		time.Sleep(errorPeriod - 2*safeCheckPeriod)

		value, err = renewable.Get()
		assert.Nil(t, value)
		assert.Equal(t, fmt.Errorf("%d", i), err)
		assert.Equal(t, iterCount*2-i*2-2, len(results))

		time.Sleep(2 * safeCheckPeriod)
	}
}

func asyncTestRenewable(
	t *testing.T,
	createRenewable func(produce ProduceFunc) (Renewable, error),
	testOnce func(t *testing.T, createRenewable func(produce ProduceFunc) (Renewable, error)),
) {
	const iterCount = 10

	for i := 0; i < iterCount; i++ {
		testOnce(t, createRenewable)

		if t.Failed() {
			t.FailNow()
		}
	}
}

func asyncTestRenewableOnce(t *testing.T, createRenewable func(produce ProduceFunc) (Renewable, error)) {
	const (
		iterCount = 10
		minNumCPU = 2
	)

	var (
		busyGetRoutineCount    = 1
		valueCheckRoutineCount = 2
	)

	if !assert.GreaterOrEqual(t, runtime.NumCPU(), minNumCPU, "insufficient CPUs to run the test properly") {
		t.FailNow()
	}

	if runtime.NumCPU() >= minNumCPU*2 {
		busyGetRoutineCount *= 2
		valueCheckRoutineCount *= 2
	}

	var iter uint64 = 0
	renewable, errCreate := createRenewable(defaultProduceFunc(t, &iter, iterCount))
	require.NoError(t, errCreate, "failed creating renewable")

	time.Sleep(safeCheckPeriod)
	assert.Equal(t, uint64(0), iter)

	ctx, cancel := context.WithCancel(context.Background())

	var gwg sync.WaitGroup
	gwg.Add(busyGetRoutineCount)

	for i := 0; i < busyGetRoutineCount; i++ {
		go func() {
			defer gwg.Done()

			assert.NotPanics(t, func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						_, _ = renewable.Get()
					}
				}
			})
		}()
	}

	var cwg sync.WaitGroup
	cwg.Add(valueCheckRoutineCount)

	for i := 0; i < valueCheckRoutineCount; i++ {
		go func() {
			defer cwg.Done()

			assert.NotPanics(t, func() {
				for i := 0; i < iterCount; i++ {
					value, err := renewable.Get()
					assert.Equal(t, uint64(i), value)
					assert.Nil(t, err)

					if i > 0 {
						time.Sleep(defaultPeriod - 2*safeCheckPeriod)
					} else {
						time.Sleep(defaultPeriod - safeCheckPeriod)
					}

					value, err = renewable.Get()
					assert.Equal(t, uint64(i), value)
					assert.Nil(t, err)

					time.Sleep(2 * safeCheckPeriod)

					value, err = renewable.Get()
					assert.Nil(t, value)
					assert.Equal(t, fmt.Errorf("%d", i), err)

					time.Sleep(errorPeriod - 2*safeCheckPeriod)

					value, err = renewable.Get()
					assert.Nil(t, value)
					assert.Equal(t, fmt.Errorf("%d", i), err)

					if i < iterCount-1 {
						time.Sleep(2 * safeCheckPeriod)
					}
				}
			})
		}()
	}

	cwg.Wait()
	cancel()
	gwg.Wait()
}

func defaultProduceFunc(t *testing.T, iter *uint64, iterCount uint64) ProduceFunc {
	return func(ctx context.Context) (value interface{}, err error) {
		i := atomic.LoadUint64(iter)
		if i > (iterCount+1)*2 {
			assert.FailNow(t, fmt.Sprintf("iter is unexpectedly large: %d", i))
		}
		defer func() {
			if !atomic.CompareAndSwapUint64(iter, i, i+1) {
				assert.FailNow(t, "iter has been modified concurrently")
			}
		}()

		if i%2 == 0 {
			return i / 2, nil
		} else {
			return nil, fmt.Errorf("%d", (i-1)/2)
		}
	}
}
