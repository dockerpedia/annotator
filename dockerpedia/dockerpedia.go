package dockerpedia

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
	manifestV1 "github.com/docker/distribution/manifest/schema1"
	registryClient "github.com/dockerpedia/docker-registry-client/registry"

	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/klar"
	"github.com/gin-gonic/gin"
)

type v1Compatibility struct {
	ID             	string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	ContainerConfig struct { Cmd []string } `json:"container_config,omitempty"`
	Author    string `json:"author,omitempty"`
	ThrowAway bool   `json:"throwaway,omitempty"`
}

type SoftwareImage struct {
	Image      string                     `form:"image" json:"image" binding:"required" predicate:"image"`
	Version    string                     `form:"tag" json:"tag" binding:"required" predicate:"rdfs:name"`
	Size       int64                      `json:"size" predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/size"`
	Features   []*clair.Feature           `json:"features" predicate:"hasPackage"`
	ManifestV1 *manifestV1.SignedManifest `json:"manifest" predicate:"manifest"`
	History    []v1Compatibility		  `json:"history"`
}


type Dockerfile struct {
	Steps     []string `json:steps`
}

var dockerurl string = "https://registry-1.docker.io/"
var username string = "" // anonymous
var password string = "" // anonymous

func NewRepository(c *gin.Context) {
	clientRegistry := registryClient.New(dockerurl, username, password)
	var newImage SoftwareImage
	if err := c.ShouldBindJSON(&newImage); err == nil {
		//Get tag size
		size, err := clientRegistry.TagSize(newImage.Image, newImage.Version)
		if err != nil {
			log.Printf(newImage.Image, "Unable to the get size of the image %s:%s", newImage.Version)
		}
		newImage.Size = size

		//Get manifest
		manifest, errManifest := clientRegistry.Manifest(newImage.Image, newImage.Version)
		if errManifest != nil {
			log.Printf("Unable to the get manifest of the image %s:%s", newImage.Image, newImage.Version)
		}
		newImage.ManifestV1 = manifest

		//Get features
		newImage.Features, err = klar.Run(newImage.Image)

		ConvertTriples(newImage)

		if errManifest != nil {
			log.Printf("Unable to the get features of the image %s:%s", newImage.Image, newImage.Version)
		}

		newImage.History = parseManifestV1Compatibility(newImage.ManifestV1)

		c.JSON(http.StatusOK, gin.H{
			"result": newImage,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func parseManifestV1Compatibility(manifest *manifestV1.SignedManifest) []v1Compatibility {
	var v1Items []v1Compatibility
	for _, history := range manifest.History {
		data, err := json.Marshal(history.V1Compatibility)
		if err != nil {
			log.Fatal("JSON marshaling failed")
		}

		var v1Item v1Compatibility
		var val []byte = []byte(data)
		s, _ := strconv.Unquote(string(val))
		err = json.Unmarshal([]byte(s), &v1Item)
		if err != nil {
			log.Fatalf("JSON marshaling failed %s", err)
		}

		v1Items = append(v1Items, v1Item)
	}
	return v1Items
}