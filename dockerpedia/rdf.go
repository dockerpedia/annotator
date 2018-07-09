package dockerpedia

import (
	tstore "github.com/wallix/triplestore"
	"fmt"
	"net/http"
	"bytes"
	"log"
	"io/ioutil"
)

const (
	siteHost     string = "http://10.6.91.175:3030"
	resource	 string = "https://dockerpedia.inf.utfsm.cl/resource/"
	vocabulary	 string = "https://dockerpedia.inf.utfsm.cl/vocabulary/"
)

func sendToFuseki(buffer bytes.Buffer){
	client := &http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/test2/data", siteHost), &buffer)
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

func ConvertTriples(image SoftwareImage) {
	var bufferTag, bufferFeature bytes.Buffer

	resourceURI := fmt.Sprintf("%s%s", resource, image.Image)

	imageStruct := tstore.TriplesFromStruct(resourceURI, &image)
	enc := tstore.NewLenientNTEncoder(&bufferTag)
	enc.Encode(imageStruct...)
	sendToFuseki(bufferTag)


	for _, feature := range image.Features {
		featureStruct := tstore.TriplesFromStruct(feature.Name, feature)
		enc := tstore.NewLenientNTEncoder(&bufferFeature)
		enc.Encode(featureStruct...)
		sendToFuseki(bufferFeature)
	}

}

