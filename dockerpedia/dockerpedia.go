package dockerpedia

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	manifestV1 "github.com/docker/distribution/manifest/schema1"
	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/klar"
	registryclient "github.com/dockerpedia/docker-registry-client/registry"
	"github.com/gin-gonic/gin"
)

type v1Compatibility struct {
	ID              string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	ContainerConfig struct {
		Cmd []string
	} `json:"container_config,omitempty"`
	Author    string `json:"author,omitempty"`
	ThrowAway bool   `json:"throwaway,omitempty"`
}

type Tag struct {
	Tag        string                     `form:"tag" json:"tag" binding:"required" predicate:"name"`
	Image      string                     `form:"image" json:"image" binding:"required" predicate:"image"`
	Size       int64                      `json:"size" predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/size"`
	Features   []*clair.Feature           `json:"features" predicate:"hasPackage"`
	ManifestV1 *manifestV1.SignedManifest `json:"manifest" predicate:"manifest"`
}

var dockerurl string = "https://registry-1.docker.io/"
var username string = "" // anonymous
var password string = "" // anonymous

func NewRepository(c *gin.Context) {
	clientRegistry := registryclient.New(dockerurl, username, password)
	var json Tag
	if err := c.ShouldBindJSON(&json); err == nil {
		//Get tag size
		size, err := clientRegistry.TagSize(json.Image, json.Tag)
		if err != nil {
			log.Printf(json.Image, "Unable to the get size of the image %s:%s", json.Tag)
		}
		json.Size = size

		//Get manifest
		manifest, errManifest := clientRegistry.Manifest(json.Image, json.Tag)
		if errManifest != nil {
			log.Printf("Unable to the get manifest of the image %s:%s", json.Image, json.Tag)
		}
		json.ManifestV1 = manifest

		//Get features
		json.Features, err = klar.Run(json.Image)

		ConvertTriples(json)

		if errManifest != nil {
			log.Printf("Unable to the get features of the image %s:%s", json.Image, json.Tag)
		}

		c.JSON(http.StatusOK, gin.H{
			"result": json,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func parseManifestV1(manifest *manifestV1.SignedManifest) {
	fmt.Printf("%s\n", manifest.Name)
	fmt.Printf("%s\n", manifest.Architecture)
	fmt.Printf("%d\n", len(manifest.FSLayers))
	for index, layer := range manifest.FSLayers {
		fmt.Printf("%d,%s\n", index, layer.BlobSum)
	}
	fmt.Printf("%d\n", len(manifest.History))
	for _, history := range manifest.History {
		//fmt.Printf("%d,%s\n", index, history.V1Compatibility)
		data, err := json.Marshal(history.V1Compatibility)
		if err != nil {
			log.Fatal("JSON marshaling failed")
		}
		var historyItem v1Compatibility
		var val []byte = []byte(data)
		s, _ := strconv.Unquote(string(val))
		err2 := json.Unmarshal([]byte(s), &historyItem)
		if err2 != nil {
			log.Fatalf("JSON marshaling failed %s", err2)
		}
		fmt.Println(historyItem)
	}
}

// func dockerHub(image, tag string) (*manifestV1.SignedManifest, error) {
// 	var url string = "https://registry-1.docker.io/"
// 	var username string = "" // anonymous
// 	var password string = "" // anonymous
//
// 	hub := registry.New(url, username, password)
// 	manifest, err := hub.Manifest(image, tag)
// 	if err != nil {
// 		log.Fatal("JSON marshaling failed")
// 	}
// 	return parseManifestV1(manifest)
// }
