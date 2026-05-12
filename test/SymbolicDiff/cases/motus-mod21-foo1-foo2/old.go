package diffcase

import (
	"errors"
	"fmt"
)

func F() error {
	return fmt.Errorf("queryAbnormalMetricsConfig is nil")
}

func foo2() error {
	return errors.New("queryAbnormalMetricsConfig is nil")
}
