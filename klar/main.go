package klar

import (
	"fmt"
	"os"
	"time"

	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/docker"
)

var store = make(map[string][]*clair.Vulnerability)

func Run(imageName string) []*clair.Feature {
	clairAddr := "http://localhost:6060"
	clairTimeout := time.Duration(1) * time.Minute

	fail := func(format string, a ...interface{}) {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s\n", format), a...)
		os.Exit(1)
	}

	conf, err := newConfig(imageName, clairAddr)
	image, err := docker.NewImage(&conf.DockerConfig)
	if err != nil {
		fail("Can't parse qname: %s", err)
	}

	err = image.Pull()
	if err != nil {
		fail("Can't pull image: %s", err)
	}

	var fs []*clair.Feature
	//obtain the packages using Annotate
	for _, ver := range []int{1, 3} {
		c := clair.NewClair(clairAddr, ver, clairTimeout)
		fs, err = c.Annotate(image)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to analyze using API v%d: %s\n", ver, err)
		} else {
			if !conf.JSONOutput {
				fmt.Printf("Got results from Clair API v%d\n", ver)
			}
			break
		}
	}
	if err != nil {
		fail("Failed to analyze, exiting")
	}

	fmt.Printf("Number of the features %d\n", len(fs))

	return fs
}
