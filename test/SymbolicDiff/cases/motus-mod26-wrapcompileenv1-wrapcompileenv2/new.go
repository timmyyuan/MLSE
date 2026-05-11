package diffcase

type Conf struct {
	Envs map[string]string
	Next *Conf
}

func WrapCompileEnv1(cfg *Conf, key string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, ok := cfg.Envs[key]
	if ok {
		return v, true
	}
	return "", false
}

func F(cfg *Conf, key string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, ok := cfg.Envs[key]
	if ok {
		return v, true
	}
	return "", false
}
