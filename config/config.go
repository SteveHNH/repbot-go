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

	// Env vars override or substitute config file values
	options.BindEnv("token", "REPBOT_TOKEN")
	options.BindEnv("db", "REPBOT_DB")
	options.SetDefault("db", "/data/rep.db")

	envConfig := os.Getenv("REPBOT_CONFIG")
	explicitConfig := false

	if configFile != "" {
		options.SetConfigFile(configFile)
		explicitConfig = true
	} else if envConfig != "" {
		e, _ := tilde.Expand(envConfig)
		options.SetConfigFile(e)
		explicitConfig = true
	} else {
		log.Println("searching for configfile...")
		options.SetConfigName("botconfig")
		options.AddConfigPath("$HOME/.config/repbot-go/")
		options.AddConfigPath("./config/")
	}

	options.SetConfigType("yaml")

	if err := options.ReadInConfig(); err != nil {
		if explicitConfig {
			log.Fatalf("config: failed to read explicit config file: %v", err)
		}
		log.Printf("config: no config file found, relying on environment variables: %v", err)
	}

	cfg := &Config{
		Token: options.GetString("token"),
		DB:    options.GetString("db"),
	}

	if cfg.Token == "" {
		log.Fatal("config: token is required but was not found in config file or REPBOT_TOKEN env var")
	}

	return cfg
}
