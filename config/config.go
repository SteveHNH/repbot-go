package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config contains the token for the discord bot
type Config struct {
	Token string
	DB string
}

// Get reads in the config file and returns a struct
func Get() *Config {
	options := viper.New()

	options.SetConfigFile("botconfig")
	// TODO: find out why viper seems to only be able to read config from .
	// For now just put your config wherever you run repbot from
	options.AddConfigPath(".")
	options.AddConfigPath("$HOME/.config/repbot-go/")
	options.AddConfigPath("./config/")
	options.SetConfigType("yaml")

	if err := options.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s \n", err)
	}

	return &Config{
		Token: options.GetString("token"),
		DB: options.GetString("db"),
	}
}
