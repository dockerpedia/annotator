package main

import (
	"github.com/dockerpedia/annotator/dockerpedia"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {

	router := gin.Default()

	v1 := router.Group("/api/v1/")
	{
		v1.POST("/repositories/new", dockerpedia.NewRepository)
	}

	router.Run(":8080")
}
