# Rotten Issue Tracker

The rotten issue tracker is a tool to find and keep track of issues that are _rottening_ away (having not been updated for a long time) in your github organisation and send them to your team on slack to remind people that they exist.

![example screenshot of the slack message](/screenshot.png)

## Configuration

### Environment Variables

You need to set two environment variables:

`GH_TOKEN` with a github token with at least `public_repo` scope for your organisation.

`SLACK_TOKEN` with the slack token for your bot, it needs to have the right to post in the channel you want your messages in.

### Commandline Paramteters

You also need a few command line parameters

| Name                                  | Required           | Default value           | Description |
| ------------------------------------- | ------------------ | ----------------------- | ----------- |
| channel | yes |N/A|The slack channel to post the message about the rottening issues|
| github-org | yes |N/A|The github organisation you want to check for rottening issues in|
| ignored-repos-file | no |N/A| Filename of a file in the same folder as the executable containing a list of repost to ignore. One repo name per line, LF file endings|
| rottening-threshold | no | 100| Number of days an issue can be left alone (not modified) before it is considered rotten |

## Example

`go run main.go -channel=fredrik_rotten_issue -github-org=qlik-oss -ignored-repos-file=ignored-repos.txt`