package clair

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dockerpedia/annotator/docker"
)

const EMPTY_LAYER_BLOB_SUM = "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"

// Clair is representation of Clair server
type Clair struct {
	url string
	api API
}

type API interface {
	Annotate(image *docker.Image) ([]*Feature, error)
	Analyze(image *docker.Image) ([]*Vulnerability, error)
	Push(image *docker.Image) error
}

type layer struct {
	Name       string
	Path       string
	ParentName string
	Format     string
	Features   []Feature
	Headers    headers
}

type headers struct {
	Authorization string
}

type Feature struct {
	Name            string          `json:"Name,omitempty" 
									predicate:"rdfs:label"`
	NamespaceName   string          `json:"NamespaceName,omitempty" 
									predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/hasNamespace"`
	Version         string          `json:"Version,omitempty"
									predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/hasVersion"`
	Vulnerabilities []Vulnerability `json:"Vulnerabilities"
									predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/hasVulnerability"`
	AddedBy         string          `json:"AddedBy,omitempty"
									predicate:"https://dockerpedia.inf.utfsm.cl/vocabulary/AddedBy"`
}
// Vulnerability represents vulnerability entity returned by Clair
type Vulnerability struct {
	Name           string                 `json:"Name,omitempty" predicate:"name"`
	NamespaceName  string                 `json:"NamespaceName,omitempty" predicate:"namespace"`
	Description    string                 `json:"Description,omitempty" predicate:"description"`
	Link           string                 `json:"Link,omitempty" predicate:"link"`
	Severity       string                 `json:"Severity,omitempty" predicate:"severity"`
	Metadata       map[string]interface{} `json:"Metadata,omitempty" predicate:"metadata"`
	FixedBy        string                 `json:"FixedBy,omitempty" predicate:"fixedby"`
	FixedIn        []Feature              `json:"FixedIn,omitempty" predicate:"fixedin"`
	FeatureName    string                 `json:"featureName",omitempty predicate:"featurename"`
	FeatureVersion string                 `json:"featureName",omitempty predicate:"featureversion"`
}

type layerError struct {
	Message string
}

type clairError struct {
	Message string `json:"Layer"`
}

type layerEnvelope struct {
	Layer *layer      `json:"Layer,omitempty"`
	Error *clairError `json:"Error,omitempty"`
}

// NewClair construct Clair entity using potentially incomplete server URL
// If protocol is missing HTTP will be used. If port is missing 6060 will be used
func NewClair(url string, version int, timeout time.Duration) Clair {
	api, err := newAPI(url, version, timeout)
	if err != nil {
		panic(fmt.Sprintf("cant't create API client version %d %s: %s", version, url, err))
	}

	return Clair{url, api}
}

func newLayer(image *docker.Image, index int) *layer {
	var parentName string
	if index != 0 {
		parentName = image.LayerName(index - 1)
	}

	return &layer{
		Name:       image.LayerName(index),
		Path:       strings.Join([]string{image.Registry, image.Name, "blobs", image.FsLayers[index].BlobSum}, "/"),
		ParentName: parentName,
		Format:     "Docker",
		Headers:    headers{image.Token},
	}
}

func filterEmptyLayers(fsLayers []docker.FsLayer) (filteredLayers []docker.FsLayer) {
	for _, layer := range fsLayers {
		if layer.BlobSum != EMPTY_LAYER_BLOB_SUM {
			filteredLayers = append(filteredLayers, layer)
		}
	}
	return
}

// Analyse sent each layer from Docker image to Clair and returns
// a list of found vulnerabilities
func (c *Clair) Analyse(image *docker.Image) ([]*Vulnerability, error) {
	// Filter the empty layers in image
	image.FsLayers = filterEmptyLayers(image.FsLayers)
	layerLength := len(image.FsLayers)
	if layerLength == 0 {
		fmt.Fprintf(os.Stderr, "no need to analyse image %s/%s:%s as there is no non-emtpy layer\n",
			image.Registry, image.Name, image.Tag)
		return nil, nil
	}

	if err := c.api.Push(image); err != nil {
		return nil, fmt.Errorf("push image %s/%s:%s to Clair failed: %s\n", image.Registry, image.Name, image.Tag, err.Error())
	}
	vs, err := c.api.Analyze(image)
	if err != nil {
		return nil, fmt.Errorf("analyse image %s/%s:%s failed: %s\n", image.Registry, image.Name, image.Tag, err.Error())
	}
	return vs, nil
}

// Analyse sent each layer from Docker image to Clair and returns
// a list of found vulnerabilities
func (c *Clair) Annotate(image *docker.Image) ([]*Feature, error) {
	// Filter the empty layers in image
	image.FsLayers = filterEmptyLayers(image.FsLayers)
	layerLength := len(image.FsLayers)
	if layerLength == 0 {
		fmt.Fprintf(os.Stderr, "no need to analyse image %s/%s:%s as there is no non-emtpy layer\n",
			image.Registry, image.Name, image.Tag)
		return nil, nil
	}

	if err := c.api.Push(image); err != nil {
		return nil, fmt.Errorf("push image %s/%s:%s to Clair failed: %s\n", image.Registry, image.Name, image.Tag, err.Error())
	}
	vs, err := c.api.Annotate(image)
	if err != nil {
		return nil, fmt.Errorf("analyse image %s/%s:%s failed: %s\n", image.Registry, image.Name, image.Tag, err.Error())
	}
	return vs, nil
}
