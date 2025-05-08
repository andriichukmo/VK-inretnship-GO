package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC struct {
		Addr string
	}
	Queue struct {
		Buffer int
	}
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")
	v.SetDefault("grpc.addr", ":50051")
	v.SetDefault("queue.buffer", 1024)
	v.SetDefault("shutdown_timeout", "5s")
	if err := v.ReadInConfig(); err != nil {
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
