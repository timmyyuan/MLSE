package diffcase

import (
	"errors"
	"fmt"
)

type Strategy struct {
	ID     int
	Enable bool
	Next   *Strategy
}

func GetStrategyById(id int) (*Strategy, error) {
	return &Strategy{}, nil
}

func (s *Strategy) Save() error {
	return nil
}

func F(strategyId int, rollbackEnableValue bool) error {
	if strategyId == 0 {
		return errors.New("strategy Id is zero")
	}
	strategy, err := GetStrategyById(strategyId)
	if err != nil {
		return err
	}
	strategy.Enable = rollbackEnableValue
	if err := strategy.Save(); err != nil {
		return fmt.Errorf("save failed")
	}
	return nil
}

func Rollback2(strategyId int, rollbackEnableValue bool) error {
	if strategyId == 0 {
		return errors.New("strategy Id is zero")
	}
	strategy, err := GetStrategyById(strategyId)
	if err != nil {
		return err
	}
	strategy.Enable = rollbackEnableValue
	if err := strategy.Save(); err != nil {
		return fmt.Errorf("save failed")
	}
	return nil
}
