package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config contains the token for the discord bot
type Config struct {
	Token string
}

// Get reads in the config file and returns a struct
func Get() *Config {
	viper.SetConfigFile("config")
	viper.AddConfigPath(".")
	viper.SetConfigType("yml")
	var configuration Config

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s", err)
	}

	err := viper.Unmarshal(&configuration)
	if err != nil {
		fmt.Printf("Unable to decode into struct, %v", err)
	}

	return &configuration
}
