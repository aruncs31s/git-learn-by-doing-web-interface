package models

type Developer struct {
	// Actual Name
	Name string `json:"name"`
	// Github Username
	GitHubUsername string `json:"github_username"`
	// Types , eg developer
	Category     []string `json:"category"`
	TotalCommits int      `json:"total_commits"`
}

func (d *Developer) GetProfileURL() string {
	return "https://github.com/" + d.GitHubUsername
}

func (d *Developer) GetAvatarURL() string {
	return "https://github.com/" + d.GitHubUsername + ".png"
}

func (d *Developer) GetMyDetails() *Developer {
	var details *Developer
	details.Name = "Arun CS"
	details.GitHubUsername = "aruncs31s"
	details.Category = []string{"developer"}
	return details
}
