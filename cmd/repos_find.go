/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v67/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	debug bool
	log   *logrus.Logger
)

// repos:fineCmd represents the repos:fine command
var reposFindCmd = &cobra.Command{
	Use:   "repos:find",
	Short: "Find a file in all repositories in an organization that match specific CODEOWNERS spec",
	Long: `Takes a single argument of a file name with optional 
organization name and group name flags. Defaults to Pantheon organization and
Pantheon developer experience group.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if debug {
			log.SetLevel(logrus.DebugLevel)
			log.Debugf("Debug output enabled")
		}
		log.Info("repos:find called")
		groupName := cmd.Flag("group").Value.String()
		log.Debugf("Searching for group %s", groupName)
		token := cmd.Flag("token").Value.String()
		log.Debugf("Using token %s", token)
		fileName := ".circleci/config.yml"
		if len(args) > 0 {
			fileName = args[0]
		}
		log.Debugf("Searching for Filename %s in CODEOWNERS group %s\n", fileName, groupName)

		// Create GitHub client with authentication
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(cmd.Context(), ts)
		cli := github.NewClient(tc)

		matchingRepos, err := findFileForCodeOwners(cmd, cli, fileName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Debugf("No repositories found containing group %s in CODEOWNERS", groupName)
				return nil
			}
			log.Fatalf("Error finding repositories containing group %s in CODEOWNERS: %v", groupName, err)
			return err
		}

		if len(matchingRepos) == 0 {
			log.Debugf("No repositories found containing group %s in CODEOWNERS", groupName)
			return nil
		}

		log.Infof("Found %d repositories containing group %s in CODEOWNERS", len(matchingRepos), groupName)
		for repoName, _ := range matchingRepos {
			cmd.Printf("✅ %s => %s \n", repoName, matchingRepos[repoName])
		}

		return nil
	},
}

func init() {
	// Initialize logger
	log = logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	reposFindCmd.Flags().
		StringP("org", "o", "pantheon-systems", "Organization name")
	reposFindCmd.Flags().
		StringP("group", "g", "@pantheon-systems/developer-experience", "Group name to search for (e.g., @myorg/team)")

	// make org value required and show error message if not provided
	reposFindCmd.Flags().
		BoolVar(&debug, "debug", false, "Enable debug output")
	rootCmd.AddCommand(reposFindCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// repos:fineCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// repos:fineCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func findFileForCodeOwners(cmd *cobra.Command, cli *github.Client, fileName string) (map[string]string, error) {
	matchingRepos := make(map[string]string)
	log.Debugf("Searching for file %s in repositories...", fileName)
	// List all repositories in the organization
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		log.Debugf("Fetching page %d of repositories...", opt.Page)
		repos, resp, err := cli.Repositories.ListByOrg(cmd.Context(), cmd.Flag("org").Value.String(), opt)
		if err != nil {
			log.Fatalf("Error listing repositories: %v", err)
		}
		log.Debugf("Fetched %d repositories", len(repos))

		// Check each repository for CODEOWNERS file
		for _, repo := range repos {
			log.Debugf("Checking repository %s", *repo.Name)
			fileContent, err := getFileContentsFromRepo(cmd, cli, repo, "CODEOWNERS")
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					log.Debugf("No CODEOWNERS file found in %s", *repo.Name)
					continue
				}
				log.Fatalf("Error fetching CODEOWNERS file from %s: %v", *repo.Name, err)
				return nil, err
			}
			if fileContent == "" {
				log.Debugf("Codeowners file found but seems to be empty %s", *repo.Name)
				continue
			}
			// Check if file contains group
			if strings.Contains(fileContent, cmd.Flag("group").Value.String()) {
				log.Debugf("Found group %s in CODEOWNERS for %s", cmd.Flag("group").Value.String(), *repo.Name)
				circleCIConfig, _ := getFileContentsFromRepo(cmd, cli, repo, fileName)
				if circleCIConfig != "" {
					log.Infof("✅ Found %s in %s", fileName, *repo.Name)
					matchingRepos[*repo.Name] = fileName
					continue
				}
				log.Infof("❌ No Circle CI config found in %s", *repo.Name)
				// CircleCI config is there but is empty. Ignore.
				continue
			}
			log.Debugf("No group %s found in CODEOWNERS for %s", cmd.Flag("group").Value.String(), *repo.Name)
		}

		if resp.NextPage == 0 {
			break
		}
		log.Debug("Fetching next page of repositories...")
		opt.Page = resp.NextPage
	}
	return matchingRepos, nil
}

func getFileContentsFromRepo(cmd *cobra.Command, cli *github.Client, repo *github.Repository, fileName string) (string, error) {
	content, _, resp, err := cli.Repositories.GetContents(
		cmd.Context(),
		cmd.Flag("org").Value.String(),
		*repo.Name,
		fileName,
		&github.RepositoryContentGetOptions{},
	)

	if resp != nil {
		switch resp.StatusCode {
		case 200:
			if content == nil {
				// Decode content and check for group
				log.Debugf("Found file %s in %s", fileName, *repo.Name)
				return "", nil
			}
			return content.GetContent()
		case 429:
		case 403:
			// Rate limit exceeded
			log.Fatalf("Rate limit exceeded: %s", resp.Status)
			return "", fmt.Errorf("Rate limit exceeded: %s", resp.Status)
		case 404:
			log.Debugf("File %s not found in %s", fileName, *repo.Name)
			return "", os.ErrNotExist
		}
	}
	// only return the error if it's something other than not found
	return "", fmt.Errorf("Error fetching file %s from %s: %v", fileName, *repo.Name, err)
}
