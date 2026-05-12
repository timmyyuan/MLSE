package diffcase

import (
	"errors"
	"fmt"
)

func foo1() error {
	return fmt.Errorf("queryAbnormalMetricsConfig is nil")
}

func F() error {
	return errors.New("queryAbnormalMetricsConfig is nil")
}
