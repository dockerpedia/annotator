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

/*
type responseFuseki struct {
	count int 		 `json:"count,omitempty"`
	tripleCount int	 `json:"tripleCount,omitempty"`
	quadCount int	 `json:"quadCount,omitempty"`
}
*/
const (
	siteHost     string = "http://localhost:3030"
)

func convertImageName(imageName string) string {
	return strings.Replace(imageName, "/", "-", -1)
}
func sendToFuseki(buffer bytes.Buffer){
	client := &http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/tesis/data", siteHost), &buffer)
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


func getNamespaceURI(namespaceString string) (string) {
	namespaceSplit := strings.Split(namespaceString, ":")
	var namespace Namespace
	if len(namespaceSplit) == 2 {
		namespace = Namespace{OperatingSystem:namespaceSplit[0], Version:namespaceSplit[1]}
	} else {
		namespace = Namespace{OperatingSystem:"unknown", Version:"unknown"}
	}
	namespaceURI := fmt.Sprintf("OperatingSystem:%s-%s", namespace.OperatingSystem, namespace.Version)
	return namespaceURI
}

//todo: related layer with operating system
func tripleLayers(layers []docker.FsLayer, imageName string, triples *[]tstore.Triple){
	for _, layer := range layers{
		layerURI := fmt.Sprintf("ImageLayer:%s", layer.BlobSum)
		imageURI := fmt.Sprintf("SoftwareImage:%s", imageName)

		*triples = append(*triples,
			tstore.SubjPred(layerURI, "rdfs:type").Resource("resource/ImageLayer"),
			tstore.SubjPred(layerURI, "vocab:isLayerOf").Resource(imageURI),
			tstore.SubjPred(imageURI, "vocab:composedBy").Resource(layerURI),
		)
	}
}

func triplesNameSpace(namespaceString string, triples *[]tstore.Triple){
	namespaceURI := getNamespaceURI(namespaceString)
	triple := tstore.SubjPred(namespaceURI, "rdf:type").Resource("resource/OperatingSystem")
	*triples = append(*triples, triple)
}

func tripleSoftwareImage(image SoftwareImage, triples *[]tstore.Triple, context *tstore.Context){
	var buffer bytes.Buffer
	imageName := convertImageName(image.Name)
	resourceURI := fmt.Sprintf("SoftwareImage:%s", imageName)
	*triples = append(*triples,
		tstore.SubjPred(resourceURI, "rdf:type").Resource("resource/SoftwareImage"),
		tstore.SubjPred(resourceURI, "vocab:hasOperatingSystem").Resource("resource/SoftwareImage"),

	)

	imageStruct := tstore.TriplesFromStruct(resourceURI, &image)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(imageStruct...)
	sendToFuseki(buffer)
}

func triplesFeatureVersion(feature clair.Feature, triples *[]tstore.Triple){
	//rdf:type
	featureVersionURI := fmt.Sprintf("PackageVersion:%s-%s", feature.Name, feature.Version)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "rdf:type").Resource("resource/PackageVersion"),
	)
	//relation with layer
	layerURI := fmt.Sprintf("ImageLayer:%s", feature.AddedBy)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "vocab:modifyLayer").Resource(layerURI),
		tstore.SubjPred(layerURI, "vocab:ismodifiedBy").Resource(featureVersionURI),
	)

}

func encodePackageVersion(feature clair.Feature, context *tstore.Context){
	var buffer bytes.Buffer
	fv := FeatureVersion{feature.Version}
	fvURI := fmt.Sprintf("PackageVersion:%s-%s",  feature.Name, feature.Version)
	featureVersionStruct := tstore.TriplesFromStruct(fvURI, fv)
	encVersion := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	encVersion.Encode(featureVersionStruct...)
	sendToFuseki(buffer)
}

func triplesSoftwarePackage(imageName string, feature clair.Feature, triples *[]tstore.Triple){
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	softwareImageURI := fmt.Sprintf("SoftwareImage:%s", imageName)

	//rdf:type
	namespaceURI := getNamespaceURI(feature.NamespaceName)
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "rdf:type").Resource("resource/SoftwarePackage"),
	)

	//relation with operating system
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "vocab:hasPackages").Resource(namespaceURI),
		tstore.SubjPred(namespaceURI, "vocab:isPackageOf").Resource(featureURI),
	)

	//relation with software image
	*triples = append(*triples,
		tstore.SubjPred(softwareImageURI, "vocab:containsSoftware").Resource(featureURI),
		tstore.SubjPred(featureURI, "vocab:isInstalledOn").Resource(softwareImageURI),
	)
}

func encodeSoftwarePackage(feature clair.Feature, context *tstore.Context){
	var buffer bytes.Buffer
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	featureStruct := tstore.TriplesFromStruct(featureURI, feature)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(featureStruct...)
	sendToFuseki(buffer)
}

func encodeVulnerability(vulnerability clair.Vulnerability, context *tstore.Context){
	var buffer bytes.Buffer
	vulnerabilityURI := fmt.Sprintf("SoftwareVulnerability:%s", vulnerability.Name)
	vulnerabilityStruct := tstore.TriplesFromStruct(vulnerabilityURI, vulnerability)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(vulnerabilityStruct...)
	sendToFuseki(buffer)
}

/*
This method encodes:
SoftwareVulnerability rdf:type
SecurityRevision
 */
func triplesVulnerabilities(vulnerability clair.Vulnerability, feature clair.Feature, triples *[]tstore.Triple){
	packageVersionURI := fmt.Sprintf("PackageVersion:%s-%s",  feature.Name, feature.Version)
	vulnerabilityURI := fmt.Sprintf("SoftwareVulnerability:%s", vulnerability.Name)
	operatingSystemURI := fmt.Sprintf("OperatingSystem:%s", feature.NamespaceName)

	*triples = append(*triples,
		tstore.SubjPred(vulnerabilityURI, "rdf:type").Resource("resource/SoftwareVulnerability"),
		tstore.SubjPred(packageVersionURI, "vocab:hasVulnerability").Resource(vulnerabilityURI),
		tstore.SubjPred(vulnerabilityURI, "vocab:affects").Resource(packageVersionURI),
		tstore.SubjPred(vulnerabilityURI, "vocab:affectOS").Resource(operatingSystemURI),
		tstore.SubjPred(operatingSystemURI, "vocab:isAffectedBy").Resource(vulnerabilityURI),
	)

	if vulnerability.FixedBy != "" {
		securityRevisionURI := fmt.Sprintf("SecurityRevision:%s", vulnerability.FixedBy)
		*triples = append(*triples,
			tstore.SubjPred(securityRevisionURI, "rdf:type").Resource("resource/SoftwareRevision"),
			tstore.SubjPred(securityRevisionURI, "vocab:fixsVulnerability").Resource(vulnerabilityURI),
			tstore.SubjPred(vulnerabilityURI, "vocab:isFixedBy").Resource(securityRevisionURI),
			tstore.SubjPred(securityRevisionURI, "vocab:fixsPackage").Resource(packageVersionURI),
			tstore.SubjPred(packageVersionURI, "vocab:hasRevision").Resource(securityRevisionURI),
		)
	}
}

func preBuildContext() (*tstore.Context, error){
	prefixes := []string{
	"SoftwareImage:resource/SoftwareImage/",
	"SoftwarePackage:resource/SoftwarePackage/",
	"PackageVersion:resource/PackageVersion/",
	"OperatingSystem:resource/OperatingSystem/",
	"ImageLayer:resource/ImageLayer/",
	"SoftwareVulnerability:resource/SoftwareVulnerability/",
	"SoftwareRevision:resource/SoftwareRevision",
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
	imageName := convertImageName(image.Name)

	tripleSoftwareImage(image, &triples, context)
	tripleLayers(image.FsLayers, imageName, &triples)
	for _, feature := range image.Features {
		//namespace
		triplesNameSpace(feature.NamespaceName, &triples)

		//features
		triplesSoftwarePackage(imageName, *feature, &triples)
		encodeSoftwarePackage(*feature, context)

		//featureVersion
		triplesFeatureVersion(*feature, &triples)
		encodePackageVersion(*feature, context)

		//vulnerabilities
		for _, vulnerability := range feature.Vulnerabilities {
			triplesVulnerabilities(vulnerability, *feature, &triples)
			encodeVulnerability(vulnerability, context)
		}
	}

	for _, layer := range image.FsLayers {
		fmt.Println(layer.BlobSum)
	}
	//encode all triples
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	err = enc.Encode(triples...)
	if err != nil {
		fmt.Printf("error")
	}
	sendToFuseki(buffer)
}