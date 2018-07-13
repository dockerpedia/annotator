package dockerpedia

import (
	tstore "github.com/wallix/triplestore"
	"fmt"
	"net/http"
	"bytes"
	"log"
	"io/ioutil"
	"strings"
	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/docker"

)

type responseFuseki struct {
	count int 		 `json:"count,omitempty"`
	tripleCount int	 `json:"tripleCount,omitempty"`
	quadCount int	 `json:"quadCount,omitempty"`
}

const (
	siteHost     string = "http://10.6.91.175:3030"
	resource	 string = "https://dockerpedia.inf.utfsm.cl/resource/"
	vocabulary	 string = "https://dockerpedia.inf.utfsm.cl/vocabulary/"
	softwarePackage string = "SoftwarePackage"
	packageVersion string =  "PackageVersion"
)

func sendToFuseki(buffer bytes.Buffer){
	client := &http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/test3/data", siteHost), &buffer)
	req.Header.Set("Content-Type", "text/plain")

	if err != nil {
		log.Println(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}


	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func buildContext(prefixes []string, base string) (*tstore.Context, error) {
	context := tstore.RDFContext
	for _, prefix := range prefixes {
		splits := strings.SplitN(prefix, ":", 2)
		if splits[0] == "" || splits[1] == "" {
			return context, fmt.Errorf("invalid prefix format: '%s'. expected \"prefix:http://my.uri\"", prefix)
		}
		context.Prefixes[splits[0]] = splits[1]
	}
	context.Base = base
	return context, nil
}

func tripleLayers(layers []docker.FsLayer, imageName string, triples *[]tstore.Triple){
	for _, layer := range layers{
		layerURI := fmt.Sprint("ImageLayer:%s", layer.BlobSum)
		imageURI := fmt.Sprint("SoftwareImage:%s", imageName)

		*triples = append(*triples,
			tstore.SubjPred(layerURI, "rdfs:type").Resource("resource/ImageLayer"),
			tstore.SubjPred(layerURI, "vocab:isLayerOf").Resource(imageURI),
			tstore.SubjPred(imageURI, "vocab:composedBy").Resource(layerURI),
		)
	}
}
func appendNameSpace(namespace string, triples *[]tstore.Triple){
	namespaceURI := fmt.Sprintf("OperatingSystem:%s", namespace)
	triple := tstore.SubjPred(namespaceURI, "rdf:type").Resource("resource/OperatingSystem")
	featureSplit := strings.Split(namespace, ":")
	if len(featureSplit) == 2 {
		namespace := Namespace{OperatingSystem:featureSplit[0], Version:featureSplit[1]}
		namespaceURI = fmt.Sprintf("OperatingSystem:%s-%s", namespace.OperatingSystem, namespace.Version)
	} else {
		namespaceURI = fmt.Sprintf("OperatingSystem:unkwown")
	}
	*triples = append(*triples, triple)
}


func tripleSoftwareImage(image SoftwareImage, triples *[]tstore.Triple, context *tstore.Context){
	var buffer bytes.Buffer
	resourceURI := fmt.Sprintf("SoftwareImage:%s", image.Name)
	*triples = append(*triples,
		tstore.SubjPred(resourceURI, "rdf:type").Resource("resource/SoftwareImage"),
	)
	imageStruct := tstore.TriplesFromStruct(resourceURI, &image)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(imageStruct...)
	sendToFuseki(buffer)
}

func appendFeature(feature clair.Feature, triples *[]tstore.Triple, context *tstore.Context){
	var buffer bytes.Buffer
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "rdf:type").Resource("resource/SoftwarePackage"),
	)
	featureStruct := tstore.TriplesFromStruct(featureURI, feature)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(featureStruct...)
	sendToFuseki(buffer)
}

func appendFeatureVersion(feature clair.Feature, triples *[]tstore.Triple, context *tstore.Context){
	var buffer bytes.Buffer
	fv := FeatureVersion{feature.Version}

	//rdf:type
	featureVersionURI := fmt.Sprintf("PackageVersion:%s-%s", feature.Name, fv.Version)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "rdf:type").Resource("resource/PackageVersion"),
	)

	//relation with layer
	layerURI := fmt.Sprint("ImageLayer:%s", feature.AddedBy)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "vocab:modifyLayer").Resource(layerURI),
		tstore.SubjPred(layerURI, "vocab:ismodifiedBy").Resource(featureVersionURI),
	)

	//from struct
	fvURI := fmt.Sprintf("PackageVersion:%s-%s",  feature.Name, fv.Version)
	featureVersionStruct := tstore.TriplesFromStruct(fvURI, fv)
	encVersion := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	encVersion.Encode(featureVersionStruct...)
	sendToFuseki(buffer)
}

func preBuildContext() (*tstore.Context, error){
	prefixes := []string{
	"SoftwareImage:resource/SoftwareImage/",
	"SoftwarePackage:resource/SoftwarePackage/",
	"PackageVersion:resource/PackageVersion/",
	"OperatingSystem:resource/OperatingSystem/",
	"ImageLayer:resource/ImageLayer/",
	"vocab:vocab#",
	}
	context, err := buildContext(prefixes, "http://dockerpedia.inf.utfsm.cl/" )
	if err != nil {
	log.Printf("Error")
	}

	return context, err

}
func AnnotateFuseki(image SoftwareImage) {

	var buffer bytes.Buffer
	var triples []tstore.Triple
	context, err := preBuildContext()
	if err != nil {
		log.Println("Failed build the context")
	}
	tripleSoftwareImage(image, &triples, context)
	tripleLayers(image.FsLayers, &triples)
	for _, feature := range image.Features {
		//namespace
		appendNameSpace(feature.NamespaceName, &triples)

		//features
		appendFeature(*feature, &triples, context)

		//featureVersion
		appendFeatureVersion(*feature, &triples, context)

	}
	//encode all triples
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	err = enc.Encode(triples...)
	if err != nil {
		fmt.Printf("error")
	}
	sendToFuseki(buffer)
}