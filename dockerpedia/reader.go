package dockerpedia

import (
	"bytes"
	"net/http"
	"fmt"
	"log"
	"io/ioutil"
	tstore "github.com/wallix/triplestore"
	"strings"
)


func readFromFuseki() ([]byte, error) {
	client := &http.Client{}
	var raw = []byte(`query=
PREFIX vocab: <http://dockerpedia.inf.utfsm.cl/vocab#>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
CONSTRUCT {
	<http://dockerpedia.inf.utfsm.cl/resource/SoftwareImage/mosorio-dispel4py> vocab:containsSoftware ?l
} WHERE {
<http://dockerpedia.inf.utfsm.cl/resource/SoftwareImage/mosorio-dispel4py> vocab:containsSoftware  ?p .
  ?p rdfs:label ?l
}
`)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/test3/sparql", siteHost), bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/plain")
	if err != nil {
		log.Println(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	return body, err
}

func convertResponse() []tstore.Triple{
	data, err  := readFromFuseki()
	if err != nil{
		log.Println("error reading the data")
	}
	fmt.Println(data)
	r:= bytes.NewReader(data)
	dec := tstore.NewDatasetDecoder(tstore.NewLenientNTDecoder, r)

	triples, err := dec.Decode()
	if err != nil {
		log.Println(err)
	}
	return triples

}

func generatePackagesName(subject string, stringsSlice *[]string) {
	new := strings.Replace(subject, "http://dockerpedia.inf.utfsm.cl/vocab", "", -1)
	*stringsSlice = append(*stringsSlice, new)
}


func getSoftwarePackages(){
	triples := convertResponse()
	packages := []string{}
	src := tstore.NewSource()
	src.Add(triples...)
	snap := src.Snapshot()
	softwareTriples := snap.WithPredicate("http://dockerpedia.inf.utfsm.cl/vocab#containsSoftware")
	for _, tri := range softwareTriples {
		literal, _ := tri.Object().Literal()
		generatePackagesName(literal.Value(), &packages)
		packages = append(packages, literal.Value())
	}
	fmt.Println(packages)
	//writeAptGet(packages)
}

func detectFormatPackages(namespace string){
	switch namespace {
	//Debian 6, 7, 8, unstable namespaces
	//Ubuntu 12.04, 12.10, 13.04, 14.04, 14.10, 15.04, 15.10, 16.04 namespaces
	case "dpkg":
		fmt.Println(namespace)
	//CentOS 5, 6, 7 namespaces
	//Oracle Linux 5, 6, 7 namespaces
	case "rpm":
		fmt.Println(namespace)
	//Alpine 3.3, Alpine 3.4, Alpine 3.5 namespaces
	case "apk":
		fmt.Println(namespace)
	}
}

func writeAptGet(packages []string){
	if len(packages) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(`apt-get update && apt-get install -y \` )
		for i := 0; i < len(packages) - 1; i++{
			buffer.WriteString(fmt.Sprintf("%s \t", packages[i]))
		}
		buffer.WriteString(fmt.Sprintf("%s", packages[len(packages)-1]))
		fmt.Println(buffer.String())
	}
}

func writeYum(packages []string){
	if len(packages) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(`yum install -y \` )
		for i := 0; i < len(packages) - 1; i++{
			buffer.WriteString(fmt.Sprintf("%s \t", packages[i]))
		}
		buffer.WriteString(fmt.Sprintf("%s", packages[len(packages)-1]))
		fmt.Println(buffer.String())
	}

}