package utils

import (
	"os/exec"
	"regexp"
	"strings"
)

// GitUserInfo contains extracted git user information
type GitUserInfo struct {
	Username string
	Email    string
	Source   string // "git-config", "remote-origin", "not-found"
}

// GetGitUsername attempts to extract the GitHub username from git configuration
func GetGitUsername() *GitUserInfo {
	info := &GitUserInfo{
		Source: "not-found",
	}

	// Try to get from git remote origin URL first (most reliable for GitHub username)
	if username := getGitUsernameFromRemote(); username != "" {
		info.Username = username
		info.Source = "remote-origin"
		return info
	}

	// Fallback to git config user.name
	if username := getGitConfigValue("user.name"); username != "" {
		info.Username = username
		info.Source = "git-config"
	}

	// Also get email for reference
	if email := getGitConfigValue("user.email"); email != "" {
		info.Email = email
		// If we got username from config but not remote, try to extract from email
		if info.Username == "" && strings.Contains(email, "@") {
			emailUser := strings.Split(email, "@")[0]
			info.Username = emailUser
			info.Source = "git-config"
		}
	}

	return info
}

// getGitUsernameFromRemote extracts username from git remote origin URL
func getGitUsernameFromRemote() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	remoteURL := strings.TrimSpace(string(output))

	// Handle GitHub SSH URLs: git@github.com:username/repo.git
	sshRegex := regexp.MustCompile(`git@github\.com:([^/]+)/`)
	if matches := sshRegex.FindStringSubmatch(remoteURL); len(matches) > 1 {
		return matches[1]
	}

	// Handle GitHub HTTPS URLs: https://github.com/username/repo.git
	httpsRegex := regexp.MustCompile(`https://github\.com/([^/]+)/`)
	if matches := httpsRegex.FindStringSubmatch(remoteURL); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// getGitConfigValue gets a value from git config
func getGitConfigValue(key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "status")
	err := cmd.Run()
	return err == nil
}
