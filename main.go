package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack"
)

//The Issue struct describes a github issue
type Issue struct {
	IssueURL   string      `json:"html_url"`
	Title      string      `json:"title"`
	Repository Repository  `json:"repository"`
	Body       string      `json:"body"`
	State      string      `json:"state"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	IsPR       interface{} `json:"pull_request"`
}

//The Repository struct describes the repo
type Repository struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	RepoURL string `json:"html_url"`
}
type issueSlice []Issue

func (s issueSlice) Less(i, j int) bool { return s[i].UpdatedAt.Before(s[j].UpdatedAt) }
func (s issueSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s issueSlice) Len() int           { return len(s) }

var (
	client       = &http.Client{}
	ignoredRepos = make(map[string]bool)
)

func handleError(err error) {
	if err != nil {
		log.Fatalf(err.Error())
	}
}

func fetchOldIssues(githubToken string, githubOrg string, rotteningTreshold time.Duration) issueSlice {

	//läs från env variabler
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/orgs/%s/issues?filter=all&state=open&per_page=500", githubOrg), nil)
	handleError(err)
	req.Header.Add("Authorization", fmt.Sprintf("token %s", githubToken))

	resp, err := client.Do(req)
	handleError(err)
	body, err := ioutil.ReadAll(resp.Body)
	handleError(err)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatal(fmt.Sprintf("Recived statuscode %d and body '%s'\nMake sure that the github token you are using have the public_repo scope", resp.StatusCode, string(body)))
	}

	var issues issueSlice
	err = json.Unmarshal(body, &issues)
	handleError(err)

	return filterAndSortIssues(issues, rotteningTreshold)
}

func filterAndSortIssues(issues issueSlice, rotteningTreshold time.Duration) []Issue {

	rotteningIssues := issueSlice{}
	for _, issue := range issues {

		timeSinceLastUpdateInHours := time.Since(issue.UpdatedAt)
		//check that the issue is not a pull request
		if issue.IsPR == nil && timeSinceLastUpdateInHours > rotteningTreshold*24*time.Hour && !ignoredRepos[issue.Repository.Name] {
			rotteningIssues = append(rotteningIssues, issue)
		}
	}

	sort.Sort(rotteningIssues)
	return rotteningIssues
}

func formattedWeeklyIssues(issues issueSlice, numberOfIssuesThisWeek int, numberOfIssuesLastWeek int, rotteningTreshold int) (string, []string) {

	if numberOfIssuesThisWeek == 0 {
		return fmt.Sprintf("No rottening issues! Great work :fiestaparrot:\n Last week we had *%d* rottening issues.", numberOfIssuesLastWeek), nil
	}

	slackMessage := fmt.Sprintf("Currently we have *%d* issues that have not updated for over *%d* days\n", numberOfIssuesThisWeek, rotteningTreshold)

	if numberOfIssuesThisWeek < numberOfIssuesLastWeek {
		slackMessage += fmt.Sprintf("That is *%d* fewer than last week :slightly_smiling_face:", numberOfIssuesLastWeek-numberOfIssuesThisWeek)
	} else if numberOfIssuesThisWeek > numberOfIssuesLastWeek {
		slackMessage += fmt.Sprintf("That is *%d* more than last week :white_frowning_face:", numberOfIssuesThisWeek-numberOfIssuesLastWeek)
	} else {
		slackMessage += "That is the same number as last week :neutral_face:"
	}

	slackMessage += "\n\n*Rottening issues:* \n\n"

	var attachmentTexts []string
	attachmentText := ""
	for _, issue := range issues {

		//only count whole days
		daysAgo := int(math.Floor(time.Since(issue.UpdatedAt).Seconds() / 86400))
		fixedTitle := strings.Replace(issue.Title, "`", "", 100)
		attachmentText += fmt.Sprintf("• <%s|%s> in the <%s|%s> repo\nLast updated *%d* days ago"+"\n\n", issue.IssueURL, fixedTitle, issue.Repository.RepoURL, issue.Repository.Name, daysAgo)

		//split into several attachments since slack has a max length of the attachment.
		if len(attachmentText) > 3500 {
			attachmentTexts = append(attachmentTexts, attachmentText)
			attachmentText = ""
		}
	}
	//add the eventual last attachment
	if attachmentText != "" {
		attachmentTexts = append(attachmentTexts, attachmentText)
	}
	return slackMessage, attachmentTexts
}

func fetchEnvironmentVariableOrQuit(environmentVariableName string) string {
	environmentVariable, found := os.LookupEnv(environmentVariableName)
	if !found {
		log.Fatalf(fmt.Sprintf("%s environment variable needs to be set.", environmentVariableName))
	}
	return environmentVariable
}

func populateIgnoredRepos(ignoredReposFilePath string) {
	if ignoredReposFilePath != "" {

		file, err := os.Open(ignoredReposFilePath)
		handleError(err)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			ignoredRepos[scanner.Text()] = true
		}
	}
}

func main() {

	githubToken := fetchEnvironmentVariableOrQuit("GH_TOKEN")
	slackToken := fetchEnvironmentVariableOrQuit("SLACK_TOKEN")

	var slackChannel string
	flag.StringVar(&slackChannel, "channel", "", "The slack channel to post to")
	var githubOrg string
	flag.StringVar(&githubOrg, "github-org", "", "The github organisation to check for rotten issues in")
	var ignoredReposFilePath string
	flag.StringVar(&ignoredReposFilePath, "ignored-repos-path", "", "The relative path to a file containing a list of repos to ignore")
	var rotteningTreshold int
	flag.IntVar(&rotteningTreshold, "rottening-treshold", 100, "The treshold in days for when an issue is considered rotten")

	flag.Parse()
	if slackChannel == "" {
		log.Fatalf("You need to specify which slack channel to send the message to. Like this '-channel=my-slack-channel'")
	}
	if githubOrg == "" {
		log.Fatalf("You need to specify which github-organisation to check for rottening issues. Like this '-github-org=my-github-org'")
	}
	populateIgnoredRepos(ignoredReposFilePath)

	api := slack.New(slackToken)
	issues := fetchOldIssues(githubToken, githubOrg, time.Duration(rotteningTreshold))

	readBytes, err := ioutil.ReadFile("issues-last-week.txt")
	handleError(err)

	numberOfIssuesLastWeek, err := strconv.Atoi(string(readBytes))
	numberOfIssuesThisWeek := len(issues)
	slackeMessage, attachmentTexts := formattedWeeklyIssues(issues, numberOfIssuesThisWeek, numberOfIssuesLastWeek, rotteningTreshold)

	attachments := []slack.Attachment{}
	for index, attachmentText := range attachmentTexts {
		var Pretext string
		if index == 0 {
			Pretext = ""
		} else {
			Pretext = "*Next batch of newer but still rottening issues: *"
		}
		attachment := slack.Attachment{
			Text:       attachmentText,
			Pretext:    Pretext,
			MarkdownIn: []string{"text"},
		}
		attachments = append(attachments, attachment)
	}

	err = ioutil.WriteFile("issues-last-week.txt", []byte(strconv.Itoa(numberOfIssuesThisWeek)), os.ModePerm)
	handleError(err)
	_, _, err = api.PostMessage(slackChannel, slackeMessage, slack.PostMessageParameters{Markdown: true, Attachments: attachments})
	handleError(err)
}
