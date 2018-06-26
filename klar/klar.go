package klar

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dockerpedia/annotator/clair"
	"github.com/dockerpedia/annotator/docker"
	"github.com/dockerpedia/annotator/utils"
)

//Used to represent the structure of the whitelist YAML file
type vulnerabilitiesWhitelistYAML struct {
	General []string
	Images  map[string][]string
}

//Map structure used for ease of searching for whitelisted vulnerabilites
type vulnerabilitiesWhitelist struct {
	General map[string]bool            //key: CVE and value: true
	Images  map[string]map[string]bool //key: image name and value: [key: CVE and value: true]
}

const (
	optionClairOutput           = "CLAIR_OUTPUT"
	optionClairAddress          = "CLAIR_ADDR"
	optionKlarTrace             = "KLAR_TRACE"
	optionClairThreshold        = "CLAIR_THRESHOLD"
	optionClairTimeout          = "CLAIR_TIMEOUT"
	optionDockerTimeout         = "DOCKER_TIMEOUT"
	optionJSONOutput            = "JSON_OUTPUT"
	optionDockerUser            = "DOCKER_USER"
	optionDockerPassword        = "DOCKER_PASSWORD"
	optionDockerToken           = "DOCKER_TOKEN"
	optionDockerInsecure        = "DOCKER_INSECURE"
	optionRegistryInsecure      = "REGISTRY_INSECURE"
	optionWhiteListFile         = "WHITELIST_FILE"
	optionFeaturesOutput        = "FEATURES_OUTPUT"
	optionVulnerabilitiesOutput = "VULNERABILITIES_OUTPUT"
)

var priorities = []string{"Unknown", "Negligible", "Low", "Medium", "High", "Critical", "Defcon1"}

func parseOutputPriority() (string, error) {
	clairOutput := priorities[0]
	outputEnv := os.Getenv(optionClairOutput)
	if outputEnv != "" {
		output := strings.Title(strings.ToLower(outputEnv))
		correct := false
		for _, sev := range priorities {
			if sev == output {
				clairOutput = sev
				correct = true
				break
			}
		}

		if !correct {
			return "", fmt.Errorf("Clair output level %s is not supported, only support %v\n", outputEnv, priorities)
		}
	}
	return clairOutput, nil
}

func parseIntOption(key string) int {
	val := 0
	valStr := os.Getenv(key)
	if valStr != "" {
		val, _ = strconv.Atoi(valStr)
	}
	return val
}

func parseBoolOption(key string) bool {
	val := false
	if envVal, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		val = envVal
	}
	return val
}

type jsonOutput struct {
	LayerCount      int
	Vulnerabilities map[string][]*clair.Vulnerability
}

type config struct {
	ClairAddr       string
	ClairOutput     string
	Threshold       int
	JSONOutput      bool
	ClairTimeout    time.Duration
	DockerConfig    docker.Config
	WhiteListFile   string
	Features        bool
	Vulnerabilities bool
}

func newConfig(imageName, clairAddr string) (*config, error) {
	if clairAddr == "" {
		return nil, fmt.Errorf("Clair address must be provided\n")
	}

	if os.Getenv(optionKlarTrace) != "" {
		utils.Trace = true
	}

	clairOutput, err := parseOutputPriority()
	if err != nil {
		return nil, err
	}

	clairTimeout := parseIntOption(optionClairTimeout)
	if clairTimeout == 0 {
		clairTimeout = 1
	}

	dockerTimeout := parseIntOption(optionDockerTimeout)
	if dockerTimeout == 0 {
		dockerTimeout = 1
	}

	return &config{
		ClairAddr:       clairAddr,
		ClairOutput:     clairOutput,
		Threshold:       parseIntOption(optionClairThreshold),
		JSONOutput:      parseBoolOption(optionJSONOutput),
		Features:        parseBoolOption(optionFeaturesOutput),
		Vulnerabilities: parseBoolOption(optionVulnerabilitiesOutput),

		ClairTimeout:  time.Duration(clairTimeout) * time.Minute,
		WhiteListFile: os.Getenv(optionWhiteListFile),
		DockerConfig: docker.Config{
			ImageName:        imageName,
			User:             os.Getenv(optionDockerUser),
			Password:         os.Getenv(optionDockerPassword),
			Token:            os.Getenv(optionDockerToken),
			InsecureTLS:      parseBoolOption(optionDockerInsecure),
			InsecureRegistry: parseBoolOption(optionRegistryInsecure),
			Timeout:          time.Duration(dockerTimeout) * time.Minute,
		},
	}, nil
}
