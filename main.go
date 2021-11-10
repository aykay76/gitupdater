package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-redis/redis"
	"github.com/google/go-github/v39/github"
	"github.com/rs/xid"
	"golang.org/x/oauth2"
)

var (
	githubowner  = *flag.String("github-owner", os.Getenv("GITHUB_OWNER"), "Owner of the repository in GitHub")
	githubrepo   = *flag.String("github-repo", os.Getenv("GITHUB_REPOSITORY"), "GitHub repository where templates and dashboards are stored")
	githubbranch = *flag.String("github-branch", os.Getenv("GITHUB_BRANCH"), "Name of branch where dashboards will get created")
	githubpath   = *flag.String("github-path", os.Getenv("GITHUB_PATH"), "The path in the repo where dashboards will be stored")
	githubuser   = *flag.String("github-user", os.Getenv("GITHUB_USER"), "Full name of committer of changes to repository")
	githubemail  = *flag.String("github-email", os.Getenv("GITHUB_EMAIL"), "E-mail address of committer of changes to rpeository")
	githubpat    = *flag.String("github-pat", os.Getenv("GITHUB_ACCESS_TOKEN"), "Personal access token for GitHub")
	help         = flag.Bool("help", false, "do you need help with the command line?")
	redisClient  *redis.Client
)

func envOverride(key string, value string) string {
	temp, set := os.LookupEnv(key)
	if set {
		fmt.Println("Overriding", key, "from environment variable")
		return temp
	}

	return value
}

func main() {
	flag.CommandLine.Parse(os.Args[1:])

	var ro redis.Options
	ro.Addr = os.Getenv("REDIS_ADDR")

	redisClient = redis.NewClient(&ro)
	_, err := redisClient.Ping().Result()
	if err != nil {
		log.Fatal("Unable to connect to Redis, cannot proceed", err)
	}
	log.Println("Connected to Redis server")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubpat},
	)
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	stream := "dashboards"
	consumersGroup := "dashboards-consumer-group"
	err = redisClient.XGroupCreate(stream, consumersGroup, "0").Err()
	if err != nil {
		log.Println(err)
	}

	// generate a new reader group
	uniqueID := xid.New().String()
	for {
		entries, err := redisClient.XReadGroup(&redis.XReadGroupArgs{
			Group:    consumersGroup,
			Consumer: uniqueID,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    0,
			NoAck:    false,
		}).Result()
		if err != nil {
			log.Fatal(err)
		}

		for i := 0; i < len(entries[0].Messages); i++ {
			messageID := entries[0].Messages[i].ID
			values := entries[0].Messages[i].Values
			eventDescription := fmt.Sprintf("%v", values["whatHappened"])
			filename := fmt.Sprintf("%v", values["filename"])
			payload := fmt.Sprintf("%v", values["payload"])

			fmt.Println(payload)

			if eventDescription == "dashboard created" {
				// Upload a new file to the repository
				fmt.Println("Uploading new dashboard to GitHub repository", githubowner, "/", githubrepo)
				options := &github.RepositoryContentFileOptions{
					Message:   github.String("autograf added in response to namespace"),
					Content:   []byte(payload),
					Branch:    &githubbranch,
					Committer: &github.CommitAuthor{Name: &githubuser, Email: &githubemail},
				}

				contentResponse, response, err := githubClient.Repositories.CreateFile(context.TODO(), githubowner, githubrepo, filepath.Join(githubpath, filename), options)
				if err == nil {
					fmt.Println(response.Response.StatusCode)
					fmt.Println(response.Response.Status)
					fmt.Println(response.Response.Header)
					fmt.Println(*contentResponse.SHA)
				} else {
					fmt.Println(err)
				}
				redisClient.XAck(stream, consumersGroup, messageID)
			}
		}
	}
}
