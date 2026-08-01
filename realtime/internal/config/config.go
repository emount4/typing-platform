package config

type Config struct {
	Port         string
	Env          string
	WSHubTimeout int
}

func NewConfig() (*Config, error) {
	return &Config{
		Port:         ":8080",
		Env:          "development",
		WSHubTimeout: 60,
	}, nil
}

func NewConfigMust() *Config {
	config, err := NewConfig()
	if err != nil {
		panic(err)
	}
	return config
}
