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
	"github.com/dockerpedia/annotator/docker"
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


//http://dockerpedia.inf.utfsm.cl/resource/SoftwareImage/{id}
type SoftwareImage struct {
	Name      string                     `form:"image" json:"image" binding:"required" predicate:"rdfs:label"`
	Version    string                     `form:"tag" json:"tag" binding:"required" predicate:"vocab:version"`
	Size       int64                      `json:"size" predicate:"vocab:size"`
	Features   []*clair.Feature           `json:"features"`
	ManifestV1 *manifestV1.SignedManifest `json:"manifest"`
	History    []v1Compatibility		  `json:"history"`
	FsLayers   []docker.FsLayer
}

//http://dockerpedia.inf.utfsm.cl/resource/DockerFile/{id}
type Dockerfile struct {
	Steps     []string `json:steps`
}

//http://dockerpedia.inf.utfsm.cl/resource/PackageVersion/{id}
type FeatureVersion struct {
	Version string  `json:"Version,omitempty" predicate:"rdfs:label"`
}

type Namespace struct {
	OperatingSystem string `json:"NamespaceName,omitempty" predicate:"vocab:operatingSystem"`
	Version string `json:"Version,omitempty" predicate:"vocab:operatingSystemVersion"`
}


type Layer struct {
	Name       string  `predicate:"rdfs:label"`
	ParentName string
}

var dockerurl string = "https://registry-1.docker.io/"
var username string = "" // anonymous
var password string = "" // anonymous

func NewRepository(c *gin.Context) {
	clientRegistry := registryClient.New(dockerurl, username, password)
	var newImage SoftwareImage
	if err := c.ShouldBindJSON(&newImage); err == nil {
		//Get tag size
		var dockerImage *docker.Image

		size, err := clientRegistry.TagSize(newImage.Name, newImage.Version)
		if err != nil {
			log.Printf(newImage.Name, "Unable to the get size of the image %s:%s", newImage.Version)
		}
		newImage.Size = size

		//Get manifest
		manifest, errManifest := clientRegistry.Manifest(newImage.Name, newImage.Version)
		if errManifest != nil {
			log.Printf("Unable to the get manifest of the image %s:%s", newImage.Name, newImage.Version)
		}
		newImage.ManifestV1 = manifest

		//Get features
		newImage.Features, dockerImage, err = klar.DockerAnalyze(newImage.Name)
		newImage.FsLayers = dockerImage.FsLayers
		AnnotateFuseki(newImage)

		if errManifest != nil {
			log.Printf("Unable to the get features of the image %s:%s", newImage.Name, newImage.Version)
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