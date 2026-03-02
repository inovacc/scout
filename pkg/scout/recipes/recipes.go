package recipes

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/inovacc/scout/pkg/scout/recipe"
)

//go:embed presets
var presetsFS embed.FS

// Preset describes an available recipe preset.
type Preset struct {
	ID          string `json:"id"`
	Service     string `json:"service"`
	Description string `json:"description"`
	File        string `json:"file"`
}

var catalog = []Preset{
	{ID: "slack-channels", Service: "slack", Description: "List Slack workspace channels", File: "presets/slack-channels.json"},
	{ID: "slack-messages", Service: "slack", Description: "Extract Slack channel messages", File: "presets/slack-messages.json"},
	{ID: "discord-channels", Service: "discord", Description: "List Discord server channels", File: "presets/discord-channels.json"},
	{ID: "discord-messages", Service: "discord", Description: "Extract Discord channel messages", File: "presets/discord-messages.json"},
	{ID: "teams-channels", Service: "teams", Description: "List Microsoft Teams channels", File: "presets/teams-channels.json"},
	{ID: "reddit-posts", Service: "reddit", Description: "Extract Reddit subreddit posts", File: "presets/reddit-posts.json"},
	{ID: "gmail-inbox", Service: "gmail", Description: "Extract Gmail inbox messages", File: "presets/gmail-inbox.json"},
	{ID: "outlook-inbox", Service: "outlook", Description: "Extract Outlook inbox messages", File: "presets/outlook-inbox.json"},
	{ID: "linkedin-feed", Service: "linkedin", Description: "Extract LinkedIn feed posts", File: "presets/linkedin-feed.json"},
	{ID: "linkedin-profile", Service: "linkedin", Description: "Extract LinkedIn profile data", File: "presets/linkedin-profile.json"},
	{ID: "jira-issues", Service: "jira", Description: "Extract Jira project issues", File: "presets/jira-issues.json"},
	{ID: "confluence-pages", Service: "confluence", Description: "List Confluence space pages", File: "presets/confluence-pages.json"},
	{ID: "twitter-feed", Service: "twitter", Description: "Extract Twitter/X feed tweets", File: "presets/twitter-feed.json"},
	{ID: "youtube-channel", Service: "youtube", Description: "Extract YouTube channel videos", File: "presets/youtube-channel.json"},
	{ID: "youtube-search", Service: "youtube", Description: "Extract YouTube search results", File: "presets/youtube-search.json"},
	{ID: "notion-pages", Service: "notion", Description: "List Notion workspace pages", File: "presets/notion-pages.json"},
	{ID: "gdrive-files", Service: "gdrive", Description: "List Google Drive files", File: "presets/gdrive-files.json"},
	{ID: "sharepoint-files", Service: "sharepoint", Description: "List SharePoint document library files", File: "presets/sharepoint-files.json"},
	{ID: "amazon-search", Service: "amazon", Description: "Extract Amazon search results", File: "presets/amazon-search.json"},
	{ID: "amazon-product", Service: "amazon", Description: "Extract Amazon product details", File: "presets/amazon-product.json"},
	{ID: "gmaps-search", Service: "gmaps", Description: "Extract Google Maps search results", File: "presets/gmaps-search.json"},
	{ID: "salesforce-cases", Service: "salesforce", Description: "Extract Salesforce cases", File: "presets/salesforce-cases.json"},
	{ID: "grafana-dashboards", Service: "grafana", Description: "List Grafana dashboards", File: "presets/grafana-dashboards.json"},
	{ID: "cloud-aws-console", Service: "cloud", Description: "Extract AWS IAM users", File: "presets/cloud-aws-console.json"},
	{ID: "cloud-gcp-console", Service: "cloud", Description: "Extract GCP projects", File: "presets/cloud-gcp-console.json"},
	{ID: "cloud-azure-console", Service: "cloud", Description: "Extract Azure resource groups", File: "presets/cloud-azure-console.json"},
}

var index map[string]Preset

func init() {
	index = make(map[string]Preset, len(catalog))
	for _, p := range catalog {
		index[p.ID] = p
	}
}

// All returns all available presets.
func All() []Preset {
	return catalog
}

// Load reads and parses a preset recipe by ID.
func Load(id string) (*recipe.Recipe, error) {
	p, ok := index[id]
	if !ok {
		return nil, fmt.Errorf("recipes: unknown preset %q", id)
	}

	data, err := presetsFS.ReadFile(p.File)
	if err != nil {
		return nil, fmt.Errorf("recipes: read %s: %w", p.File, err)
	}

	return recipe.Parse(data)
}

// FS returns the embedded presets filesystem.
func FS() fs.FS {
	return presetsFS
}
