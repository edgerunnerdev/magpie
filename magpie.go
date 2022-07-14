package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var githubKey string

func ProcessArguments() bool {
	var githubKeyFile string

	flag.StringVar(&githubKeyFile, "k", "", "File containing GitHub API key.")
	flag.Parse()

	if githubKeyFile == "" {
		fmt.Println("Usage: magpie -k <github_key_file>.")
		return false
	}

	contents, err := ioutil.ReadFile(githubKeyFile)
	if err != nil {
		fmt.Println(err)
		return false
	}

	githubKey = strings.Trim(string(contents[:]), " \n")

	return true
}

func ToRawGithubURL(url string) string {
	return strings.Replace(url, "/blob/", "/raw/", 1)
}

func SearchGitHub(waitGroup *sync.WaitGroup, ch chan string) {
	defer waitGroup.Done()
	defer close(ch)

	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubKey},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	opts := github.SearchOptions{Sort: "forks", Order: "desc", ListOptions: github.ListOptions{Page: 1, PerPage: 100}}
	result, _, err := client.Search.Code(ctx, "SHODAN_API_KEY", &opts)
	if err != nil {
		fmt.Printf("Search.Code returned error: %v", err)
		return
	}

	for i := 0; i < len(result.CodeResults); i++ {
		rawGithubURL := ToRawGithubURL(result.CodeResults[i].GetHTMLURL())
		ch <- rawGithubURL
	}
}

func FindShodanKeys(waitGroup *sync.WaitGroup, ch chan string) {
	defer waitGroup.Done()

	uniqueKeys := make(map[string]struct{})
	var empty struct{}

	for url := range ch {
		terms := [2]string{"SHODAN_API_KEY", "shodan_api_key"}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		byteArray, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		body := string(byteArray[:])
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			for _, term := range terms {
				idx := strings.Index(line, term)
				if idx != -1 {
					keyRegex := regexp.MustCompile("[A-Za-z0-9]{32}")
					match := keyRegex.FindStringIndex(line)
					if match != nil {
						key := line[match[0]:match[1]]
						if _, ok := uniqueKeys[key]; !ok {
							uniqueKeys[key] = empty
							fmt.Println(key)
						}
					}
				}
			}
		}
	}
}

func ValidateShodanKey(waitGroup *sync.WaitGroup, ch chan string) {
	defer waitGroup.Done()
}

func main() {
	if !ProcessArguments() {
		return
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(3)

	githubResultChannel := make(chan string)
	shodanKeyChannel := make(chan string)

	go SearchGitHub(&waitGroup, githubResultChannel)
	go FindShodanKeys(&waitGroup, githubResultChannel)
	go ValidateShodanKey(&waitGroup, shodanKeyChannel)
	waitGroup.Wait()
}
