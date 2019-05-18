package dockerpedia

import (
	"bytes"
	"fmt"
	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/docker"
	tstore "github.com/wallix/triplestore"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

/*
type responseFuseki struct {
	count int 		 `json:"count,omitempty"`
	tripleCount int	 `json:"tripleCount,omitempty"`
	quadCount int	 `json:"quadCount,omitempty"`
}
*/
var	siteHost string

func convertImageName(image SoftwareImage) string {
	imageName := strings.Replace(image.Name, "/", "-", -1)
	return fmt.Sprintf("%s_%s", imageName, image.Version)
}

func sendToFuseki(buffer bytes.Buffer) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", siteHost, &buffer)
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
		context.Prefixes[splits[0]] = base + splits[1]
	}
	context.Base = base
	return context, nil
}

func getNamespaceURI(namespaceString string) (string) {
	namespaceSplit := strings.Split(namespaceString, ":")
	var namespace Namespace
	if len(namespaceSplit) == 2 {
		namespace = Namespace{OperatingSystem: namespaceSplit[0], Version: namespaceSplit[1]}
	} else {
		namespace = Namespace{OperatingSystem: "unknown", Version: "unknown"}
	}
	namespaceURI := fmt.Sprintf("OperatingSystem:%s-%s", namespace.OperatingSystem, namespace.Version)
	return namespaceURI
}

//todo: related layer with operating system
func tripleLayers(layers []docker.FsLayer, imageName string, triples *[]tstore.Triple) {
	for _, layer := range layers {
		layerURI := fmt.Sprintf("ImageLayer:%s", layer.BlobSum)
		imageURI := fmt.Sprintf("SoftwareImage:%s", imageName)

		*triples = append(*triples,
			tstore.SubjPred(layerURI, "rdfs:type").Resource("vocab:ImageLayer"),
			tstore.SubjPred(layerURI, "vocab:isLayerOf").Resource(imageURI),
			tstore.SubjPred(imageURI, "vocab:composedBy").Resource(layerURI),
		)
	}
}

func triplesNameSpace(namespaceString string, triples *[]tstore.Triple) {
	namespaceURI := getNamespaceURI(namespaceString)
	triple := tstore.SubjPred(namespaceURI, "rdf:type").Resource("vocab:OperatingSystem")
	*triples = append(*triples, triple)
}

func checkAddTriple(source, predicate, object string, triples *[]tstore.Triple) {
	if object != "" {
		*triples = append(*triples,
			tstore.SubjPred(source, predicate).Resource(object),
		)
	}
}
func triplesSoftwareImage(softwareImage SoftwareImage, triples *[]tstore.Triple, context *tstore.Context) {
	var buffer bytes.Buffer
	identifier := convertImageName(softwareImage)

	softwareImageURI := fmt.Sprintf("SoftwareImage:%s", identifier)

	*triples = append(*triples,
		tstore.SubjPred(softwareImageURI, "rdf:type").Resource("vocab:SoftwareImage"),
		tstore.SubjPred(softwareImageURI, "vocab:imageIdentifier").StringLiteral(identifier),
		tstore.SubjPred(softwareImageURI, "vocab:tag").StringLiteral(softwareImage.Version),
		tstore.SubjPred(softwareImageURI, "vocab:size").IntegerLiteral(int(softwareImage.Size)),
	)

	//checkAddTriple(softwareImageURI,"vocab:description", softwareImage.Labels.Description, triples)
	//checkAddTriple(softwareImageURI,"vocab:name", softwareImage.Labels.Name, triples)
	//checkAddTriple(softwareImageURI,"vocab:usage", softwareImage.Labels.Usage, triples)
	//checkAddTriple(softwareImageURI,"vocab:url", softwareImage.Labels.Url, triples)
	//checkAddTriple(softwareImageURI,"vocab:vcsUrl", softwareImage.Labels.VcsUrl, triples)
	//checkAddTriple(softwareImageURI,"vocab:vcsRef", softwareImage.Labels.VcsRef, triples)
	//checkAddTriple(softwareImageURI,"vocab:vendor", softwareImage.Labels.Vendor, triples)
	//checkAddTriple(softwareImageURI,"vocab:schema-version", softwareImage.Labels.SchemaVersion, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmd", softwareImage.Labels.DockerCmd, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmdDevel", softwareImage.Labels.DockerCmdDevel, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmdTest", softwareImage.Labels.DockerCmdTest, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmdDebug", softwareImage.Labels.DockerCmdDebug, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmdHelp", softwareImage.Labels.DockerCmdHelp, triples)
	//checkAddTriple(softwareImageURI,"vocab:dockerCmdParams", softwareImage.Labels.DockerCmdParams, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmd", softwareImage.Labels.RktCmd, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmdDevel", softwareImage.Labels.RktCmdDevel, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmdTest", softwareImage.Labels.RktCmdTest, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmdDebug", softwareImage.Labels.RktCmdDebug, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmdHelp", softwareImage.Labels.RktCmdHelp, triples)
	//checkAddTriple(softwareImageURI,"vocab:rktCmdParams", softwareImage.Labels.RktCmdParams, triples)

	imageStruct := tstore.TriplesFromStruct(softwareImageURI, &softwareImage)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(imageStruct...)
	sendToFuseki(buffer)
}

func triplesFeatureVersion(imageName string, feature clair.Feature, triples *[]tstore.Triple) {
	//rdf:type
	featureVersionLabel := fmt.Sprintf("%s_%s", feature.Name, feature.Version)
	featureVersionURI := fmt.Sprintf("PackageVersion:%s-%s", feature.Name, feature.Version)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "rdfs:label").StringLiteral(featureVersionLabel),
		tstore.SubjPred(featureVersionURI, "rdf:type").Resource("vocab:PackageVersion"),
	)
	//relation with ImageLayer
	layerURI := fmt.Sprintf("ImageLayer:%s", feature.AddedBy)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "vocab:modifyLayer").Resource(layerURI),
		tstore.SubjPred(layerURI, "vocab:ismodifiedBy").Resource(featureVersionURI),
	)

	//relation with SoftwarePackage
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	*triples = append(*triples,
		tstore.SubjPred(featureVersionURI, "vocab:isVersionOf").Resource(featureURI),
		tstore.SubjPred(featureURI, "vocab:hasVersion").Resource(featureVersionURI),
	)
	//relation with SoftwareImage
	softwareImageURI := fmt.Sprintf("SoftwareImage:%s", imageName)
	*triples = append(*triples,
		tstore.SubjPred(softwareImageURI, "vocab:containsSoftware").Resource(featureVersionURI),
		tstore.SubjPred(featureVersionURI, "vocab:isInstalledOn").Resource(softwareImageURI),
	)

}

func triplesSoftwarePackage(imageName string, feature clair.Feature, triples *[]tstore.Triple) {
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	deploymentPlanURI := fmt.Sprintf("DeploymentPlan:%s", imageName)

	//rdf:type
	namespaceURI := getNamespaceURI(feature.NamespaceName)
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "rdf:type").Resource("vocab:SoftwarePackage"),
	)

	//relation with DeploymentPlan
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "wicus-stack:isDeploymentPlan").Resource(deploymentPlanURI),
	)

	//relation with OperatingSystem
	*triples = append(*triples,
		tstore.SubjPred(featureURI, "vocab:isPackageOf").Resource(namespaceURI),
		tstore.SubjPred(namespaceURI, "vocab:hasPackages").Resource(featureURI),
	)
}

func encodeSoftwarePackage(feature clair.Feature, context *tstore.Context) {
	var buffer bytes.Buffer
	featureURI := fmt.Sprintf("SoftwarePackage:%s", feature.Name)
	featureStruct := tstore.TriplesFromStruct(featureURI, feature)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(featureStruct...)
	sendToFuseki(buffer)
}

func encodeVulnerability(vulnerability clair.Vulnerability, context *tstore.Context) {
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
TODO: SecurityRevision
 */
func triplesVulnerabilities(vulnerability clair.Vulnerability, feature clair.Feature, triples *[]tstore.Triple) {
	packageVersionURI := fmt.Sprintf("PackageVersion:%s-%s", feature.Name, feature.Version)
	vulnerabilityURI := fmt.Sprintf("SoftwareVulnerability:%s", vulnerability.Name)
	operatingSystemURI := fmt.Sprintf("OperatingSystem:%s", feature.NamespaceName)
	securityRevisionURI := fmt.Sprintf("SecurityRevision:%s", vulnerability.FixedBy)

	//SoftwareVulnerability - PackageVersion
	*triples = append(*triples,
		tstore.SubjPred(vulnerabilityURI, "rdf:type").Resource("vocab:SoftwareVulnerability"),
		tstore.SubjPred(vulnerabilityURI, "vocab:affectsPackageVersion").Resource(packageVersionURI),
		tstore.SubjPred(packageVersionURI, "vocab:hasVulnerability").Resource(vulnerabilityURI),
	)

	//SoftwareVulnerability - OperatingSystem
	*triples = append(*triples,
		tstore.SubjPred(vulnerabilityURI, "vocab:affectOS").Resource(operatingSystemURI),
		tstore.SubjPred(operatingSystemURI, "vocab:isAffectedBy").Resource(vulnerabilityURI),
	)

	if vulnerability.FixedBy != "" {
		//SecurityRevision - Vulnerability
		*triples = append(*triples,
			tstore.SubjPred(securityRevisionURI, "rdf:type").Resource("vocab:SecurityRevision"),
			tstore.SubjPred(securityRevisionURI, "vocab:fixVulnerability").Resource(vulnerabilityURI),
			tstore.SubjPred(vulnerabilityURI, "vocab:isFixedBy").Resource(securityRevisionURI),
		)
		//SecurityRevision - PackageVersion
		*triples = append(*triples,
			tstore.SubjPred(securityRevisionURI, "vocab:fixPackage").Resource(packageVersionURI),
			tstore.SubjPred(packageVersionURI, "vocab:hasRevision").Resource(securityRevisionURI),
		)
	}
}

func preBuildContext() (*tstore.Context, error) {
	prefixes := []string{
		"vocab:vocab#",
		"resource:resource/",
		"SoftwareImage:resource/SoftwareImage/",
		"SoftwarePackage:resource/SoftwarePackage/",
		"PackageVersion:resource/PackageVersion/",
		"OperatingSystem:resource/OperatingSystem/",
		"ImageLayer:resource/ImageLayer/",
		"SoftwareVulnerability:resource/SoftwareVulnerability/",
		"SoftwareRevision:resource/SoftwareRevision",
		"DeploymentPlan:resource/DeploymentPlan",
	}
	context, err := buildContext(prefixes, "http://dockerpedia.inf.utfsm.cl/")
	if err != nil {
		log.Printf("Error")
	}

	return context, err
}

func AnnotateFuseki(softwareImage SoftwareImage, endpointAddr string) bytes.Buffer {
	siteHost = endpointAddr
	var buffer bytes.Buffer
	var triples []tstore.Triple
	context, err := preBuildContext()
	if err != nil {
		log.Println("Failed build the context")
	}
	imageName := convertImageName(softwareImage)

	triplesSoftwareImage(softwareImage, &triples, context)
	tripleLayers(softwareImage.FsLayers, imageName, &triples)
	for _, feature := range softwareImage.Features {

		//featureVersion
		triplesFeatureVersion(imageName, *feature, &triples)

		//namespace
		triplesNameSpace(feature.NamespaceName, &triples)

		//features
		triplesSoftwarePackage(imageName, *feature, &triples)

		encodeSoftwarePackage(*feature, context)

		//vulnerabilities
		for _, vulnerability := range feature.Vulnerabilities {
			triplesVulnerabilities(vulnerability, *feature, &triples)
			encodeVulnerability(vulnerability, context)
		}

	}

	//encode all triples
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	err = enc.Encode(triples...)
	if err != nil {
		fmt.Printf("error")
	}
	sendToFuseki(buffer)
	return buffer
}
