package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/alecthomas/kingpin.v2"
)

const applicationVersion = "1.0.0"

var (
	app = kingpin.New("docker-registry-client", "A command-line docker registry client.")

	registryURL = app.Flag("registry", "Registry base URL (eg. https://index.docker.io)").Required().OverrideDefaultFromEnvar("REGISTRY").Short('r').URL()
	username    = app.Flag("username", "Username").OverrideDefaultFromEnvar("REGISTRY_USERNAME").Short('u').String()
	password    = app.Flag("password", "Password").OverrideDefaultFromEnvar("REGISTRY_PASSWORD").String()

	cmdDelete     = app.Command("delete", "Delete an image")
	cmdDeleteRepo = cmdDelete.Arg("repository", "Repository (eg. namespace/repo)").Required().String()
	cmdDeleteRef  = cmdDelete.Arg("reference", "Tag or digest").Required().String()

	cmdTags     = app.Command("tags", "List tags")
	cmdTagsRepo = cmdTags.Arg("repository", "Repository (eg. namespace/repo)").Required().String()
)

func init() {
	// Output log to stderr
	log.SetOutput(os.Stderr)
}

func parseAuthenticateString(v string) map[string]string {
	opts := make(map[string]string)
	s := strings.SplitN(v, " ", 2)

	parts := strings.Split(s[1], ",")

	for _, part := range parts {
		vals := strings.SplitN(part, "=", 2)
		key := vals[0]
		val := strings.Trim(vals[1], "\",")
		opts[key] = val
	}

	return opts
}

func getToken(realm, scope, service string) (string, []error) {
	request := gorequest.New()

	q := request.Get(realm).
		Param("scope", scope).
		Param("service", service)

	if *username != "" {
		q.SetBasicAuth(*username, *password)
	}

	_, body, errs := q.End()

	if errs != nil {
		return "", errs
	}

	res := make(map[string]string)
	json.Unmarshal([]byte(body), &res)
	return res["token"], nil
}

func execute(s *gorequest.SuperAgent) (gorequest.Response, string, []error) {
	// Try to execute request directly
	resp, body, errs := s.End()
	if errs != nil {
		return resp, body, errs
	}

	// Don't handle the case when the api returns another error code
	if resp.StatusCode != 401 {
		return resp, body, errs
	}

	header := resp.Header.Get("www-authenticate")
	if header == "" {
		return resp, body, []error{fmt.Errorf("Empty www-authenticate header")}
	}

	// Parse www-authenticate header
	opts := parseAuthenticateString(header)

	// Ask token to realm server
	token, errs := getToken(opts["realm"], opts["scope"], opts["service"])
	if errs != nil {
		return resp, body, []error{fmt.Errorf("Cannot retreive token: %v", errs)}
	}

	// Retry request with Authorization
	resp, body, errs = s.
		Set("Authorization", fmt.Sprintf("Bearer %s", token)).
		End()

	return resp, body, errs
}

type registry struct {
	rootURL string
}

func newRegistry() *registry {
	return &registry{rootURL: (*registryURL).String()}
}

func (r *registry) Tags(repository string) ([]string, error) {
	url := fmt.Sprintf("%s/v2/%s/tags/list", r.rootURL, repository)

	request := gorequest.New()
	q := request.Get(url)
	resp, body, errs := execute(q)
	if errs != nil {
		return nil, fmt.Errorf("%v", errs)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker API return error: %v", body)
	}

	res := struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{}

	err := json.Unmarshal([]byte(body), &res)
	if err != nil {
		return nil, err
	}

	return res.Tags, nil
}

func (r *registry) TagDigest(repository, ref string) (string, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", r.rootURL, repository, ref)

	request := gorequest.New()
	q := request.
		Get(url).
		Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, body, errs := execute(q)
	if errs != nil {
		return "", fmt.Errorf("%v", errs)
	}

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Docker API return error: %v", body)
	}

	digest := resp.Header.Get("docker-content-digest")
	if digest == "" {
		return "", fmt.Errorf("API returns empty digest")
	}

	return digest, nil
}

func (r *registry) Delete(repository, ref string) error {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", r.rootURL, repository, ref)

	request := gorequest.New()
	q := request.Delete(url)

	resp, body, errs := execute(q)
	if errs != nil {
		return fmt.Errorf("%v", errs)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Docker API returns error: %v", body)
	}

	return nil
}

func listTags(repo string) {
	r := newRegistry()
	tags, err := r.Tags(repo)
	if err != nil {
		log.Fatal(err)
	}
	if len(tags) > 0 {
		fmt.Println(strings.Join(tags, "\n"))
	}
}

func deleteTag(repo, ref string) {
	var (
		err error
	)

	r := newRegistry()

	if !(len(ref) >= 7 && ref[0:7] == "sha256:") {
		ref, err = r.TagDigest(repo, ref)
		if err != nil {
			log.Fatal("Cannot retreive tag digest", err)
		}
	}

	err = r.Delete(repo, ref)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Image %v deleted\n", ref)
}

func main() {
	app.Version(applicationVersion)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case cmdTags.FullCommand():
		listTags(*cmdTagsRepo)
	case cmdDelete.FullCommand():
		deleteTag(*cmdDeleteRepo, *cmdDeleteRef)
	}
}
