package main

import (
	"github.com/dockerpedia/annotator/dockerpedia"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"net/http"
	"github.com/spf13/viper"
	"fmt"
	"os"
)


type ServerConfig struct {
	Listen string
	Port string
}

type EndpointConfig struct {
	Server string `mapstructure:"address"`
}

type Config struct {
	Endpoint  EndpointConfig `mapstructure:"endpoint"`
	Server ServerConfig   `mapstructure:"server"`
}


func main() {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("couldn't load config: %s", err)
		os.Exit(1)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		fmt.Printf("couldn't read config: %s", err)
	}


	router := gin.Default()

	v1 := router.Group("/api/v1/")
	{
		v1.POST("/repositories/new", dockerpedia.NewRepository)

	}
	router.StaticFS("/workflows/", http.Dir("workflows/"))
	router.StaticFS("/logs/", http.Dir("logs/"))



	router.Run(c.Server.Listen)
}
