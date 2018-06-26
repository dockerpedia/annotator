package dockerpedia

import (
	"log"
	"net/http"

	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/klar"

	"github.com/gin-gonic/gin"
)

type Tag struct {
	Name     string           `form:"tag" json:"tag" binding:"required"`
	Image    string           `form:"image" json:"image" binding:"required"`
	Size     int64            `json:"size"`
	Features []*clair.Feature `json:"features"`
}

var url string = "https://registry-1.docker.io/"
var username string = "" // anonymous
var password string = "" // anonymous

func NewRepository(c *gin.Context) {

	//	hub := registry.New(url, username, password)
	var json Tag
	if err := c.ShouldBindJSON(&json); err == nil {
		//size, err := hub.TagSize(json.Repository, json.Image)
		//json.Size = size
		json.Features = klar.Run(json.Image)

		if err != nil {
			log.Fatal("Unable to the get size")
		}
		c.JSON(http.StatusOK, gin.H{
			"result": json,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

}