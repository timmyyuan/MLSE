package diffcase

import "fmt"

var defaultProductID = new(int)

func init() {
	*defaultProductID = 1
}

func GetKeyConfig(key string, defaultValue any) (any, error) {
	if key == "" {
		return nil, fmt.Errorf("key empty")
	}
	return defaultValue, nil
}

func metricErr(ok bool) error {
	if ok {
		return nil
	}
	return fmt.Errorf("metric error")
}

func addMetric(_ string, _ string, _ int, _ string, _ error) {}

func F() (bool, error) {
	val, err := GetKeyConfig("add_default_faction_type", "bool")
	if err != nil {
		addMetric("add_default_faction_type", "bool", *defaultProductID, "load", err)
		return false, err
	}
	parsed, ok := val.(bool)
	addMetric("add_default_faction_type", "bool", *defaultProductID, "parse", metricErr(ok))
	if ok {
		return parsed, nil
	}
	return false, fmt.Errorf("Parse add_default_faction_type to bool error !")
}

func GetAddDefaultFactionType2() (bool, error) {
	val, err := GetKeyConfig("add_default_faction_type", "bool")
	if err != nil {
		addMetric("add_default_faction_type", "bool", *defaultProductID, "load", err)
		return false, err
	}
	parsed, ok := val.(bool)
	addMetric("add_default_faction_type", "bool", *defaultProductID, "parse", metricErr(ok))
	if ok {
		return parsed, nil
	}
	return false, fmt.Errorf("Parse add_default_faction_type to bool error !")
}
