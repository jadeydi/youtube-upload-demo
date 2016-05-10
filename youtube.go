package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Please replace ClientSecrets to your own data.
// For more information about the client_secrets.json file format, please visit:
// https://developers.google.com/api-client-library/python/guide/aaa_client_secrets
const ClientSecrets = `{"web":{"client_id":"958753695853-qlsckqs6fio1gak25q8t7j4mcbb3rbtd.apps.googleusercontent.com","project_id":"shou-1190","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://accounts.google.com/o/oauth2/token","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","client_secret":"OzibrwCa9gy8ReFBqiaJti56","redirect_uris":["http://localhost:8080/oauth2callback"],"javascript_origins":["http://localhost"]}}`

var (
	category    = flag.String("category", "22", "Video category")
	privacy     = flag.String("privacy", "unlisted", "Video privacy status")
	description = flag.String("description", "", "Video description")
	address     = flag.String("address", "", "Video address")
)

type ClientConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris"`
	AuthURI      string   `json:"auth_uri"`
	TokenURI     string   `json:"token_uri"`
}

type Config struct {
	Installed ClientConfig `json:"installed"`
	Web       ClientConfig `json:"web"`
}

func main() {
	flag.Parse()
	if *address == "" {
		fmt.Println("You need input a vliad url")
		return
	}
	config, _ := readConfig(youtube.YoutubeScope)
	codeCh, err := startWebServer()
	if err != nil {
		fmt.Println(err)
	}
	url := config.AuthCodeURL("")
	err = openURL(url)
	if err != nil {
		fmt.Println("Visit the URL below to get a code.",
			" This program will pause until the site is visted.", err)
	} else {
		fmt.Println("Your browser has been opened to an authorization URL.",
			" This program will resume once authorization has been provided.\n")
	}
	code := <-codeCh
	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		fmt.Println(err)
	}

	if err := uploadVideo(tok.AccessToken, "Uploaded video by golang", *address); err != nil {
		fmt.Println(err)
	}
}

func uploadVideo(accessToken, title, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	token := &oauth2.Token{AccessToken: accessToken}
	config := oauth2.Config{}
	client := config.Client(oauth2.NoContext, token)

	service, err := youtube.New(client)
	if err != nil {
		return err
	}

	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: *description,
			CategoryId:  *category,
		},
		Status: &youtube.VideoStatus{PrivacyStatus: *privacy},
	}

	call := service.Videos.Insert("snippet,status", upload)
	_, err = call.Media(resp.Body, googleapi.ChunkSize(1*1024*1024)).Do()
	if err != nil {
		return err
	}
	return nil
}

func openURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:4001/").Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return err
}

func readConfig(scope ...string) (*oauth2.Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(ClientSecrets), &cfg)
	if err != nil {
		return nil, err
	}

	var redirectUri string
	if len(cfg.Web.RedirectURIs) > 0 {
		redirectUri = cfg.Web.RedirectURIs[0]
	} else if len(cfg.Installed.RedirectURIs) > 0 {
		redirectUri = cfg.Installed.RedirectURIs[0]
	} else {
		return nil, errors.New("Must specify a redirect URI in config file or when creating OAuth client")
	}

	return &oauth2.Config{
		ClientID:     cfg.Web.ClientID,
		ClientSecret: cfg.Web.ClientSecret,
		Scopes:       scope,
		RedirectURL:  redirectUri,
		Endpoint:     google.Endpoint,
	}, nil
}

func startWebServer() (codeCh chan string, err error) {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		return nil, err
	}
	codeCh = make(chan string)
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		codeCh <- code // send code to OAuth flow
		listener.Close()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Received code: %v\r\nYou can now safely close this browser window.", code)
	}))

	return codeCh, nil
}
