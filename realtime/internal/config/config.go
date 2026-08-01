package config

type Config struct {
	Port string
	Env  string
}

func NewConfig() (*Config, error) {
	return &Config{
		Port: ":8080",
		Env:  "development",
	}, nil
}

func NewConfigMust() *Config {
	config, err := NewConfig()
	if err != nil {
		panic(err)
	}
	return config
}
