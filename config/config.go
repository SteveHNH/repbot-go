package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config contains the token for the discord bot
type RepBotConfig struct {
	Token string
}

// Get reads in the config file and returns a struct
func Get() *RepBotConfig {
	options := viper.New()

	options.SetConfigFile("config")
	options.AddConfigPath(".")
	options.SetConfigType("yaml")
	options.SetDefault("token", "someplaceholder")

	if err := options.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s", err)
	}

	return &RepBotConfig{
		Token: options.GetString("token"),
	}
}
