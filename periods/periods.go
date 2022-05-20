package periods

import (
	"fmt"
	"time"
)

type Periods struct {
	Default time.Duration
	Error   time.Duration
}

func (pp Periods) Validate() error {
	if pp.Default < 0 {
		return fmt.Errorf("default period must be zero or positive, but got %v", pp.Default)
	}

	if pp.Error < 0 {
		return fmt.Errorf("error period must be zero or positive, but got %v", pp.Error)
	}

	return nil
}

func (pp Periods) Period(err error) time.Duration {
	if err != nil {
		return pp.Error
	}

	return pp.Default
}

func New(defaultPeriod, errorPeriod time.Duration) Periods {
	return Periods{Default: defaultPeriod, Error: errorPeriod}
}

func Same(period time.Duration) Periods {
	return Periods{Default: period, Error: period}
}
