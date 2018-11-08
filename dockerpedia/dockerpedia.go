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

//http://dockerpedia.inf.utfsm.cl/resource/SoftwareImage/{id}
type SoftwareImage struct {
	Name      	string        				`form:"image" json:"image" binding:"required" predicate:"rdfs:label"`
	Version    	string                     	`form:"tag" json:"tag" binding:"required"`
	Size       	int64                      	`json:"size" predicate:"vocab:size"`
	Features   	[]*clair.Feature           	`json:"features"`
	ManifestV1 	*manifestV1.SignedManifest 	`json:"manifest"`
	History    	[]v1Compatibility		  	`json:"history"`
	BaseImage 	string 						`json:"base_image"`
	Labels 		Labels						`json:"labels"`
	FsLayers   	[]docker.FsLayer
}

type Labels 	 	struct{
	BuildDate 		string 	`json:"org.label-schema.build-date,omitempty",predicate:"vocab:buildDate"`
	Description 	string  `json:"org.label-schema.description,omitempty"`
	Name 			string  `json:"org.label-schema.name,omitempty"`
	Usage 			string  `json:"org.label-schema.usage,omitempty"`
	Url 			string  `json:"org.label-schema.url,omitempty"`
	VcsUrl 			string  `json:"org.label-schema.vcs-url,omitempty"`
	VcsRef 			string  `json:"org.label-schema.vcs-ref,omitempty"`
	Vendor 			string  `json:"org.label-schema.vendor,omitempty"`
	Version 		string  `json:"org.label-schema.version,omitempty"`
	SchemaVersion 	string  `json:"org.label-schema.schema-version,omitempty"`
	DockerCmd 		string  `json:"org.label-schema.docker.cmd,omitempty"`
	DockerCmdDevel 	string  `json:"org.label-schema.docker.cmd.devel,omitempty"`
	DockerCmdTest 	string  `json:"org.label-schema.docker.cmd.test,omitempty"`
	DockerCmdDebug 	string  `json:"org.label-schema.docker.cmd.debug,omitempty"`
	DockerCmdHelp 	string  `json:"org.label-schema.docker.cmd.help,omitempty"`
	DockerCmdParams string  `json:"org.label-schema.docker.cmd.params,omitempty"`
	RktCmd 			string  `json:"org.label-schema.rkt.cmd,omitempty"`
	RktCmdDevel 	string  `json:"org.label-schema.rkt.cmd.devel,omitempty"`
	RktCmdTest 		string  `json:"org.label-schema.rkt.cmd.test,omitempty"`
	RktCmdDebug 	string  `json:"org.label-schema.rkt.cmd.debug,omitempty"`
	RktCmdHelp 		string  `json:"org.label-schema.rkt.cmd.help,omitempty"`
	RktCmdParams 	string  `json:"org.label-schema.rkt.cmd.params,omitempty"`
}

//V1Compatibility is the raw V1 compatibility information. This will contain the JSON object describing the V1 of this image.
type v1Compatibility struct {
	Architecture 	string		`json:architecture`
	ID             	string		`json:"id"`
	Parent          string		`json:"parent,omitempty"`
	Comment         string		`json:"comment,omitempty"`
	Created         time.Time	`json:"created"`
	Author    		string 		`json:"author,omitempty"`
	ThrowAway		bool   		`json:"throwaway,omitempty"`
	ContainerConfig struct {
		Cmd 		[]string 	`json:"Cmd,omitempty"`
		Image    	string 	 	`json:"Image,omitempty"`
	} `json:"container_config,omitempty"`
	Config 			struct{
		Labels 		Labels 		`json:Labels,omitempty`
	} `json:"config,omitempty"`
}

// http://dockerpedia.inf.utfsm.cl/resource/DockerFile/{id}
type Dockerfile struct {
	Steps     []string `json:steps`
}

//	Version of SoftwarePackage
// http://dockerpedia.inf.utfsm.cl/resource/PackageVersion/{id}
type FeatureVersion struct {
	Version string  `json:"Version,omitempty" predicate:"rdfs:label"`
}

//Operating system of the SoftwareImage and SoftwarePackage
type Namespace struct {
	OperatingSystem string `json:"NamespaceName,omitempty" predicate:"vocab:operatingSystem"`
	Version string `json:"Version,omitempty" predicate:"vocab:operatingSystemVersion"`
}

//Layer of DockerImage
type Layer struct {
	Name       string  `predicate:"rdfs:label"`
	ParentName string
}

// Get the params of Workflow (API)
type RequestWorkflow struct {
	Name      	string `form:"image" json:"image"`
	Version    	string `form:"tag" json:"tag"`
	PipPackages string `form:"pip_requirements" json:"pip_requirements"`
	OutputImage string `form:"output_image" json:"output_image"`
}

//Detect the operating system using the information of SoftwarePackage
func detectBaseImage(newImage SoftwareImage) (string){
	return newImage.Features[0].NamespaceName
}



var dockerurl string = "https://registry-1.docker.io/"
var username string = "" // anonymous
var password string = "" // anonymous

func NewRepository(c *gin.Context) {
	var bufferDockerfile bytes.Buffer
	clientRegistry := registryClient.New(dockerurl, username, password)
	var newImage SoftwareImage
	var request RequestWorkflow
	if err := c.ShouldBindJSON(&request); err == nil {
		//Get tag size
		var dockerImage *docker.Image
		//Get the source image from the request
		newImage.Name = request.Name
		newImage.Version = request.Version
		imageFullName := fmt.Sprintf("%s:%s", newImage.Name, newImage.Version)

		/*
		Get the info about the image
		 */
		size, err := clientRegistry.TagSize(newImage.Name, newImage.Version)
		if err != nil {
			log.Printf(newImage.Name, "Unable to the get size of the image %s:%s", newImage.Version)
		}
		newImage.Size = size

		manifest, errManifest := clientRegistry.Manifest(newImage.Name, newImage.Version)
		if errManifest != nil {
			log.Printf("Unable to the get manifest of the image %s:%s", newImage.Name, newImage.Version)
		}
		newImage.ManifestV1 = manifest

		/*
		Ask about the features of image using Klar
		 */
		newImage.Features, dockerImage, err = klar.DockerAnalyze(imageFullName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		/*
		Copy layers and history
		 */
		newImage.FsLayers = dockerImage.FsLayers
		newImage.History = parseManifestV1Compatibility(newImage.ManifestV1)


		/*
		If exists, detect the labels of the image
		*/
		if len(newImage.History) > 0 {
			newImage.Labels = newImage.History[0].Config.Labels
		}
		/*
		Detect the base image
		 */
		//if newImage.BaseImage == "" {
		//	newImage.BaseImage = detectBaseImage(newImage)
		//}
		newImage.BaseImage = "ubuntu"
		/*
		Find layer that install software and prepare the Dockerfile
		 */
		installedLines, err := findLayerInstaller(newImage.History)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"err": err,
			})
		}
		writeDockerfileContent(&bufferDockerfile, newImage.BaseImage, installedLines)

		/*
		Annotate using RDF store and build the image
		 */
		AnnotateFuseki(newImage)
		//go docker.CreateImage(request.OutputImage, dockerImage.Digest, &bufferDockerfile)

		for _, f := range newImage.Features {
			fmt.Println(f.Name, " ", f.Version)
		}
		c.JSON(http.StatusOK, gin.H{
			"dockerfile": bufferDockerfile.String(),
			"manifiest": newImage.History,
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