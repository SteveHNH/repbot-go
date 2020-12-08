package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
	tilde "gopkg.in/mattes/go-expand-tilde.v1"
)

// Config contains the token for the discord bot
type Config struct {
	Token string
	DB    string
}

// Get reads in the config file and returns a struct
func Get(configFile string) *Config {
	options := viper.New()

	envConfig := os.Getenv("REPBOT_CONFIG")

	if configFile != "" {
		options.SetConfigFile(configFile)
	} else if envConfig != "" {
		e, _ := tilde.Expand(envConfig)
		options.SetConfigFile(e)
	} else {
		log.Println("searching for configfile...")
		options.SetConfigName("botconfig")
		options.AddConfigPath("$HOME/.config/repbot-go/")
		options.AddConfigPath("./config/")
	}

	options.SetConfigType("yaml")

	if err := options.ReadInConfig(); err != nil {
		// Log the error and exit if the config file cannot be parsed
		log.Fatal(err)
	}

	return &Config{
		Token: options.GetString("token"),
		DB:    options.GetString("db"),
	}
}
