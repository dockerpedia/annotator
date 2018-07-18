package dockerpedia

import (
	"bytes"
	"net/http"
	"fmt"
	"log"
	"io/ioutil"
	tstore "github.com/wallix/triplestore"

)


func readFromFuseki() ([]byte, error) {
	client := &http.Client{}
	var raw = []byte(`query=PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
PREFIX resource: <http://dockerpedia.inf.utfsm.cl/resource/>
PREFIX vocab: <http://dockerpedia.inf.utfsm.cl/vocab#>
CONSTRUCT { ?s ?o ?p } WHERE { ?s ?o ?p }`)

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
	r:= bytes.NewReader(data)
	dec := tstore.NewDatasetDecoder(tstore.NewLenientNTDecoder, r)

	triples, err := dec.Decode()
	if err != nil {
		log.Println(err)
	}
	return triples

}

func getSoftwarePackages(){
	triples := convertResponse()
	src := tstore.NewSource()
	src.Add(triples...)
	snap := src.Snapshot()
	softwareTriples := snap.WithPredicate("http://dockerpedia.inf.utfsm.cl/vocab#modifyLayer")
	for _, tri := range softwareTriples {
		fmt.Println(tri.Subject())
	}
}
