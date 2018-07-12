package dockerpedia

import (
	tstore "github.com/wallix/triplestore"
	"fmt"
	"net/http"
	"bytes"
	"log"
	"io/ioutil"
	"strings"
	"encoding/json"
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
	var r responseFuseki
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


	b, err := ioutil.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode)
	if err != nil {
		log.Println(err)
	}
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}


	json.Unmarshal(b, &r)
	fmt.Println(r)
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





func AnnotateFuseki(image SoftwareImage) {
	prefixes := []string{
		"SoftwareImage:resource/SoftwareImage/",
		"PackageVersion:resource/PackageVersion/",
		"vocab:vocab#",
	}
	context, err := buildContext(prefixes, "http://dockerpedia.inf.utfsm.cl" )
	if err != nil {
		log.Printf("Error")
	}

	var buffer, bufferFeature bytes.Buffer

	resourceURI := fmt.Sprintf("SoftwareImage:%s", image.Image)
	imageStruct := tstore.TriplesFromStruct(resourceURI, &image)
	enc := tstore.NewLenientNTEncoderWithContext(&buffer, context)
	enc.Encode(imageStruct...)
	sendToFuseki(buffer)

	for _, feature := range image.Features {
		featureURI := fmt.Sprintf("PackageVersion:%s",  feature.Name)
		featureStruct := tstore.TriplesFromStruct(featureURI, feature)
		enc2 := tstore.NewLenientNTEncoder(&bufferFeature)
		enc2.Encode(featureStruct...)
		sendToFuseki(bufferFeature)


		fv := FeatureVersion{feature.Version}
		//featureVersionURI := fmt.Sprintf("%s%s%s", resource, packageVersion, fv.Version)
		featureVersionStruct := tstore.TriplesFromStruct(fv.Version, fv)
		encVersion := tstore.NewLenientNTEncoderWithContext(&buffer, context)
		encVersion.Encode(featureVersionStruct...)
		sendToFuseki(buffer)

	}

}