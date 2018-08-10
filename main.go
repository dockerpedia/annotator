package main

import (
	"github.com/dockerpedia/annotator/dockerpedia"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"net/http"
)

func main() {

	router := gin.Default()

	v1 := router.Group("/api/v1/")
	{
		v1.POST("/repositories/new", dockerpedia.NewRepository)

	}
	router.StaticFS("/workflows/", http.Dir("workflows/"))


	router.Run(":8081")
}
