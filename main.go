package main

import (
	"github.com/dockerpedia/annotator/internal/config"
	"github.com/dockerpedia/annotator/dockerpedia"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"log"
	"net/http"

)

func main() {
	router := gin.Default()
	configuration, err := config.New()
	if err != nil {
		log.Panicln("Configuration error", err)
	}

	v1 := router.Group("/api/v1/")
	{
		v1.POST("/repositories/new", dockerpedia.NewRepository)

	}
	router.StaticFS("/workflows/", http.Dir("workflows/"))
	router.StaticFS("/logs/", http.Dir("logs/"))

	router.Run(configuration.Server.Listen)
}

