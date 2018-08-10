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
	"fmt"
	"bytes"
	"strings"
	"errors"
)

const (
	tmpDockerDir = "./workflows/"
)

// DebianReleasesMapping translates Debian code names and class names to version numbers
var DebianReleasesMapping = map[string]string{
	// Code names
	"squeeze": "6",
	"wheezy":  "7",
	"jessie":  "8",
	"stretch": "9",
	"buster":  "10",
	"sid":     "unstable",

	// Class names
	"oldoldstable": "7",
	"oldstable":    "8",
	"stable":       "9",
	"testing":      "10",
	"unstable":     "unstable",
}

// UbuntuReleasesMapping translates Ubuntu code names to version numbers
var UbuntuReleasesMapping = map[string]string{
	"precise": "12.04",
	"quantal": "12.10",
	"raring":  "13.04",
	"trusty":  "14.04",
	"utopic":  "14.10",
	"vivid":   "15.04",
	"wily":    "15.10",
	"xenial":  "16.04",
	"yakkety": "16.10",
	"zesty":   "17.04",
	"artful":  "17.10",
	"bionic":  "18.04",
}

type v1Compatibility struct {
	Architecture 	string    `json:architecture`
	ID             	string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	ContainerConfig struct {
		Cmd 			[]string `json:"Cmd,omitempty"`
		Image    		string `json:"Image,omitempty"`
	} `json:"container_config,omitempty"`
	Author    string `json:"author,omitempty"`
	ThrowAway bool   `json:"throwaway,omitempty"`
}


//http://dockerpedia.inf.utfsm.cl/resource/SoftwareImage/{id}
type SoftwareImage struct {
	Name      	string        				`form:"image" json:"image" binding:"required" predicate:"rdfs:label"`
	Version    	string                     	`form:"tag" json:"tag" binding:"required" predicate:"vocab:version"`
	PipPackages string 					  	`form:"pip_requirements" json:"pip_requirements" predicate:"vocab:hasPipRequirements"`
	Size       	int64                      	`json:"size" predicate:"vocab:size"`
	Features   	[]*clair.Feature           	`json:"features"`
	ManifestV1 	*manifestV1.SignedManifest 	`json:"manifest"`
	History    	[]v1Compatibility		  	`json:"history"`
	BaseImage 	string 						`json:"base_image"`
	FsLayers   	[]docker.FsLayer
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

func detectBaseImage(newImage SoftwareImage) (string){
	return newImage.Features[0].NamespaceName
}

func NewRepository(c *gin.Context) {
	var bufferDockerfile bytes.Buffer
	clientRegistry := registryClient.New(dockerurl, username, password)
	var newImage SoftwareImage
	if err := c.ShouldBindJSON(&newImage); err == nil {
		//Get tag size
		var dockerImage *docker.Image
		imageFullName := fmt.Sprintf("%s:%s", newImage.Name, newImage.Version)
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
		newImage.Features, dockerImage, err = klar.DockerAnalyze(imageFullName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}


		newImage.FsLayers = dockerImage.FsLayers
		newImage.History = parseManifestV1Compatibility(newImage.ManifestV1)

		//Save namespace
		if newImage.BaseImage == "" {
			newImage.BaseImage = detectBaseImage(newImage)
		}

		installedLines, err := findLayerInstaller(newImage.History)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"err": err,
			})
		}
		go AnnotateFuseki(newImage)
		writeDockerfileContent(&bufferDockerfile, newImage.BaseImage, installedLines)
		digest := dockerImage.Digest
		dirImage := fmt.Sprintf("%s%s", tmpDockerDir, digest)
		err = docker.WriteDockerfile(dirImage, &bufferDockerfile)
		if err != nil{
			log.Printf("write dockerfile failed %s", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"dockerfile": bufferDockerfile.String(),
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}


func writeDockerfileContent(buffer *bytes.Buffer, baseImage string, installedLines []string){
	baseInstruction := fmt.Sprintf("FROM %s\n", baseImage)
	buffer.WriteString(baseInstruction)
	for _, line := range installedLines{
		installInstruction := fmt.Sprintf("RUN %s\n", line)
		buffer.WriteString(installInstruction)
	}
}


func findLayerInstaller(history []v1Compatibility) ([]string, error){
	var lines []string
	if len(history) > 0 {
		author := history[0].Author
		for _, layer := range history {
			if layer.Author == author {
				cmd := strings.Join(layer.ContainerConfig.Cmd, " ")
				cmd = strings.Replace(cmd, "/bin/sh -c ", "", -1)
				lines = append([]string{cmd}, lines...)
			}
		}
	} else {
		err := errors.New("the image doesn't has the author image, please them")
		return nil, err
	}
	return lines, nil
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