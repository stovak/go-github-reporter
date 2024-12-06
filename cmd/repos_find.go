/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v67/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// repos:fineCmd represents the repos:fine command
var reposFindCmd = &cobra.Command{
	Use:   "repos:find",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("repos:find called")
		orgName := cmd.Flag("org").Value.String()
		groupName := cmd.Flag("group").Value.String()
		token := cmd.Flag("token").Value.String()

		// Create GitHub client with authentication
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(cmd.Context(), ts)
		cli := github.NewClient(tc)

		matchingRepos := findReposForCodeOwners(cmd, cli, orgName, groupName)

		if len(matchingRepos) == 0 {
			cmd.Println("No repositories found containing group in CODEOWNERS")
			return nil
		}

		cmd.Printf("Found %d repositories containing group %s in CODEOWNERS:\n", len(matchingRepos), groupName)
		for _, repo := range matchingRepos {
			cmd.Printf("- %s\n", repo)
		}

		return nil
	},
}

func init() {
	reposFindCmd.Flags().
		StringP("org", "o", "pantheon-systems", "Organization name")
	reposFindCmd.Flags().
		StringP("group", "g", "@pantheon-systems/developer-experience", "Group name to search for (e.g., @myorg/team)")
	reposFindCmd.Flags().
		StringP("token", "t", os.Getenv("GITHUB_TOKEN"), "GitHub personal access token")
	// make org value required and show error message if not provided
	rootCmd.AddCommand(reposFindCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// repos:fineCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// repos:fineCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func findReposForCodeOwners(cmd *cobra.Command, cli *github.Client, orgName string, groupName string) []string {
	var matchingRepos []string
	// List all repositories in the organization
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		cmd.Println("Fetching repositories...")
		repos, resp, err := cli.Repositories.ListByOrg(cmd.Context(), orgName, opt)
		if err != nil {
			log.Fatalf("Error listing repositories: %v", err)
		}
		cmd.Printf("Fetched %d repositories\n", len(repos))
		// Check each repository for CODEOWNERS file
		for _, repo := range repos {
			// All codeowner files are named CODEOWNERS and in the root directory
			content, _, resp, err := cli.Repositories.GetContents(
				cmd.Context(),
				orgName,
				*repo.Name,
				"CODEOWNERS",
				&github.RepositoryContentGetOptions{},
			)

			if err != nil || resp.StatusCode == 404 {
				continue
			}

			if content != nil {
				// Decode content and check for group
				fileContent, err := content.GetContent()
				if err != nil {
					continue
				}

				if strings.Contains(fileContent, groupName) {
					// Found group in CODEOWNERS file
					cmd.Printf("Found group %s in CODEOWNERS file in repository %s\n", groupName, *repo.Name)
					matchingRepos = append(matchingRepos, *repo.Name)
					continue // Found a match in this repo, move to next repo
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		cmd.Printf("Fetching next page of repos page %d of %d...", resp.NextPage, resp.LastPage)
		opt.Page = resp.NextPage
	}
	return matchingRepos
}
