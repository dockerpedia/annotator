package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Listen string
	Port string
}

type EndpointConfig struct {
	Address string `mapstructure:"address"`
}

type ClairConfig struct {
	Address string `mapstructure:"address"`
	Port   string
}

type Config struct {
	Endpoint  	EndpointConfig `mapstructure:"endpoint"`
	Server 		ServerConfig   `mapstructure:"server"`
	Clair       ClairConfig    `mapstructure:"server"`
}

// NewConfig is used to generate a configuration instance which will be passed around the codebase
func New() (*Config, error) {
	config, err := initViper()
	return config, err
}

func initViper()  (*Config, error){
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("couldn't load config: %s", err)
		os.Exit(1)
	}
	viper.WatchConfig() // Watch for changes to the configuration file and recompile
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})

	if err = viper.ReadInConfig(); err != nil {
		log.Panicf("Error reading config file, %s", err)
	}


	var c Config
	if err := v.Unmarshal(&c); err != nil {
		fmt.Printf("couldn't read config: %s", err)
	}
	return &c, err
}