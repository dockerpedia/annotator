package docker

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dockerpedia/annotator/utils"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"errors"
	"log"
	"github.com/docker/docker/pkg/archive"
	"golang.org/x/net/context"

	"bytes"
	"path/filepath"
)

const (
	stateInitial = iota
	stateName
	statePort
	stateTag

)

const (
	workflowDir = "./workflows/"
)

// Image represents Docker image
type Image struct {
	Registry      string
	Name          string
	Tag           string
	FsLayers      []FsLayer
	Token         string
	user          string
	password      string
	client        http.Client
	Digest        string
	schemaVersion int
}

func CreateImage(outputImage, digestImage string, buf *bytes.Buffer){
	dirImage := fmt.Sprintf("%s%s", workflowDir, digestImage)
	log.Printf("creating the image using dir: %s", dirImage)
	WriteDockerfile(dirImage, buf)
	buildImagesFromFiles(outputImage, "/Users/mosorio/go/src/github.com/dockerpedia/annotator/workflows/sha256:ae8182e0c43de1ea43c9487c925b6c09ddca4737cbba444ec911c136beb27016/")
}

func buildImagesFromFiles(outputImage, dirImage string) {
	cli, err := client.NewEnvClient()
	buildCtx, err := buildContextDocker(dirImage)

	defer buildCtx.Close()
	//defer os.RemoveAll(dirImage)

	if err != nil {
		log.Printf("build context failed")
	}

	options := types.ImageBuildOptions{
		Tags:           []string{"agent:latest"},
		Remove:         true,
		ForceRemove:    true,
		PullParent:     false,
		SuppressOutput: false,
		Labels: map[string]string{
			"mosorio.app": "agent",
		},
	}

	response, err := cli.ImageBuild(context.Background(), buildCtx, options)
	if err != nil {
		log.Printf("err, %v", err)
	}
	responseOutput, err := parseBuildResponse(response)
	log.Printf(responseOutput)
	defer response.Body.Close()
}


func WriteDockerfile(path string, buf *bytes.Buffer) {
	os.Mkdir(path, 0700)
	dockerfilePath := filepath.Join(path, "Dockerfile")
	Dockerfile, err := os.OpenFile(dockerfilePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_TRUNC, 0666)
	defer Dockerfile.Close()

	_, err = Dockerfile.Write(buf.Bytes())
	if err != nil{
		log.Printf("write dockerfile failed %s", err)
	}

}

func (i *Image) LayerName(index int) string {
	s := fmt.Sprintf("%s%s", trimDigest(i.Digest),
		trimDigest(i.FsLayers[index].BlobSum))
	return s
}

func (i *Image) AnalyzedLayerName() string {
	index := len(i.FsLayers) - 1
	if i.schemaVersion == 1 {
		index = 0
	}
	return i.LayerName(index)
}

func trimDigest(d string) string {
	return strings.Replace(d, "sha256:", "", 1)
}

// FsLayer represents a layer in docker image
type FsLayer struct {
	BlobSum string
}

// ImageV1 represents a Manifest V 2, Schema 1 Docker Image
type imageV1 struct {
	SchemaVersion int
	FsLayers      []fsLayer
}

// FsLayer represents a layer in a Manifest V 2, Schema 1 Docker Image
type fsLayer struct {
	BlobSum string
}

type config struct {
	MediaType string
	Digest    string
}

// imageV2 represents Manifest V 2, Schema 2 Docker Image
type imageV2 struct {
	SchemaVersion int
	Config        config
	Layers        []layer
}

// Layer represents a layer in a Manifest V 2, Schema 2 Docker Image
type layer struct {
	Digest string
}

type Config struct {
	ImageName        string
	User             string
	Password         string
	Token            string
	InsecureTLS      bool
	InsecureRegistry bool
	Timeout          time.Duration
}

const dockerHub = "registry-1.docker.io"

var tokenRe = regexp.MustCompile(`Bearer realm="(.*?)",service="(.*?)",scope="(.*?)"`)


type BuildResponse struct {
	Stream string `json:"stream"`
	Error  string `json:"error"`
}

func extractLastOutputFromBuildResponse(response types.ImageBuildResponse) (lastOutput string, err error) {
	defer response.Body.Close()
	r, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	lastOutput = ""
	rs := strings.Split(string(r), "\n")
	i := len(rs) - 1
	for lastOutput == "" && i >= 0 {
		lastOutput = rs[i]
		i--
	}
	if lastOutput == "" {
		return "", errors.New("Could not parse container build response")
	}
	return lastOutput, nil
}

func parseBuildResponse(response types.ImageBuildResponse) (tag string, err error) {
	lastOutput, err := extractLastOutputFromBuildResponse(response)
	if err != nil {
		return "", err
	}
	var buildResponse BuildResponse

	if err := json.Unmarshal([]byte(lastOutput), &buildResponse); err != nil {
		return "", fmt.Errorf("Could not parse container build response. %s", err)
	}
	if buildResponse.Error != "" {
		return "", fmt.Errorf("Image build failed. %s", buildResponse.Error)
	}
	return strings.TrimSuffix(buildResponse.Stream, "\n"), nil
}


func buildFromTarGz() {
	cli, err := client.NewEnvClient()
	file, err := os.OpenFile("workflow.tar.gz", os.O_RDONLY, 0666)
	if err != nil {
		log.Printf("file not found", err)
		return
	}

	defer file.Close()

	options := types.ImageBuildOptions{
		Tags:           []string{"agent:latest"},
		Remove:         true,
		ForceRemove:    true,
		PullParent:     false,
		SuppressOutput: true,
		Labels: map[string]string{
			"mosorio.app": "agent",
		},
	}

	response, err := cli.ImageBuild(context.Background(), file, options)
	if err != nil {
		log.Printf("err, %v", err)
	}
	responseOutput, err := parseBuildResponse(response)
	log.Printf(responseOutput)
	defer response.Body.Close()

}

func buildContextDocker(path string) (io.ReadCloser, error) {
	content, err := archive.Tar(path, archive.Gzip)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// NewImage parses image name which could be the ful name registry:port/name:tag
// or in any other shorter forms and creates docker image entity without
// information about layers
func NewImage(conf *Config) (*Image, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.InsecureTLS},
	}
	client := http.Client{
		Transport: tr,
		Timeout:   conf.Timeout,
	}
	registry := dockerHub
	tag := "latest"
	token := ""
	var nameParts, tagParts []string
	var name, port string
	state := stateInitial
	start := 0
	for i, c := range conf.ImageName {
		if c == ':' || c == '/' || c == '@' || i == len(conf.ImageName)-1 {
			if i == len(conf.ImageName)-1 {
				// ignore a separator, include the last symbol
				i += 1
			}
			part := conf.ImageName[start:i]
			start = i + 1
			switch state {
			case stateInitial:
				if part == "localhost" || strings.Contains(part, ".") {
					// it's registry, let's check what's next =port of image name
					registry = part
					if c == ':' {
						state = statePort
					} else {
						state = stateName
					}
				} else {
					// it's an image name, if separator is /
					// next part is also part of the name
					// othrewise it's an offcial image
					if c == '/' {
						// we got just a part of name, till next time
						start = 0
						state = stateName
					} else {
						state = stateTag
						name = fmt.Sprintf("library/%s", part)
					}
				}
			case stateTag:
				tag = ""
				tagParts = append(tagParts, part)
			case statePort:
				state = stateName
				port = part
			case stateName:
				if c == ':' || c == '@' {
					state = stateTag
				}
				nameParts = append(nameParts, part)
			}
		}
	}

	if port != "" {
		registry = fmt.Sprintf("%s:%s", registry, port)
	}
	if name == "" {
		name = strings.Join(nameParts, "/")
	}
	if tag == "" {
		tag = strings.Join(tagParts, ":")
	}
	if conf.InsecureRegistry {
		registry = fmt.Sprintf("http://%s/v2", registry)
	} else {
		registry = fmt.Sprintf("https://%s/v2", registry)
	}
	if conf.Token != "" {
		token = "Basic " + conf.Token
	}

	return &Image{
		Registry: registry,
		Name:     name,
		Tag:      tag,
		user:     conf.User,
		password: conf.Password,
		Token:    token,
		client:   client,
	}, nil
}

// Pull retrieves information about layers from docker registry.
// It gets docker registry token if needed.
func (i *Image) Pull() error {
	resp, err := i.pullReq()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		if i.Token == "" {
			i.Token, err = i.requestToken(resp)
			io.Copy(ioutil.Discard, resp.Body)
		}
		if err != nil {
			return err
		}
		// try again
		resp, err = i.pullReq()
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		// try one more time by clearing the token to request it
		if resp.StatusCode == http.StatusUnauthorized {
			i.Token, err = i.requestToken(resp)
			io.Copy(ioutil.Discard, resp.Body)
			if err != nil {
				return err
			}
			// try again
			resp, err = i.pullReq()
			if err != nil {
				return err
			}
			defer resp.Body.Close()
		}
	}
	return parseImageResponse(resp, i)
}

func parseImageResponse(resp *http.Response, image *Image) error {
	contentType := resp.Header.Get("Content-Type")
	if contentType == "application/vnd.docker.distribution.manifest.v2+json" {
		var imageV2 imageV2
		if err := json.NewDecoder(resp.Body).Decode(&imageV2); err != nil {
			fmt.Fprintln(os.Stderr, "Image V2 decode error")
			return err
		}
		image.FsLayers = make([]FsLayer, len(imageV2.Layers))
		for i := range imageV2.Layers {
			image.FsLayers[i].BlobSum = imageV2.Layers[i].Digest
		}
		image.Digest = imageV2.Config.Digest
		image.schemaVersion = imageV2.SchemaVersion
	} else {
		var imageV1 imageV1
		if err := json.NewDecoder(resp.Body).Decode(&imageV1); err != nil {
			fmt.Fprintln(os.Stderr, "ImageV1 decode error")
			return err
		}
		image.FsLayers = make([]FsLayer, len(imageV1.FsLayers))
		// in schemaVersion 1 layers are in reverse order, so we save them in the same order as v2
		// base layer is the first
		for i := range imageV1.FsLayers {
			image.FsLayers[len(imageV1.FsLayers)-1-i].BlobSum = imageV1.FsLayers[i].BlobSum
		}
		image.schemaVersion = imageV1.SchemaVersion
	}
	return nil
}

func (i *Image) requestToken(resp *http.Response) (string, error) {
	authHeader := resp.Header.Get("Www-Authenticate")
	if authHeader == "" {
		return "", fmt.Errorf("Empty Www-Authenticate")
	}
	parts := tokenRe.FindStringSubmatch(authHeader)
	if parts == nil {
		return "", fmt.Errorf("Can't parse Www-Authenticate: %s", authHeader)
	}
	realm, service, scope := parts[1], parts[2], parts[3]
	var url string
	if i.user != "" {
		url = fmt.Sprintf("%s?service=%s&scope=%s&account=%s", realm, service, scope, i.user)
	} else {
		url = fmt.Sprintf("%s?service=%s&scope=%s", realm, service, scope)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't create a request")
		return "", err
	}
	if i.user != "" {
		req.SetBasicAuth(i.user, i.password)
	}
	tResp, err := i.client.Do(req)
	if err != nil {
		io.Copy(ioutil.Discard, tResp.Body)
		return "", err
	}

	defer tResp.Body.Close()
	if tResp.StatusCode != http.StatusOK {
		io.Copy(ioutil.Discard, tResp.Body)
		return "", fmt.Errorf("Token request returned %d", tResp.StatusCode)
	}
	var tokenEnv struct {
		Token string
	}

	if err = json.NewDecoder(tResp.Body).Decode(&tokenEnv); err != nil {
		fmt.Fprintln(os.Stderr, "Token response decode error")
		return "", err
	}
	return fmt.Sprintf("Bearer %s", tokenEnv.Token), nil
}

func (i *Image) pullReq() (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/manifests/%s", i.Registry, i.Name, i.Tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't create a request")
		return nil, err
	}
	if i.Token == "" {
		if i.user != "" {
			req.SetBasicAuth(i.user, i.password)
			i.Token = req.Header.Get("Authorization")
		}
	} else {
		req.Header.Set("Authorization", i.Token)
	}

	// Prefer manifest schema v2
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.v1+prettyjws")
	utils.DumpRequest(req)
	resp, err := i.client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Get error")
		return nil, err
	}
	utils.DumpResponse(resp)
	return resp, nil
}
