package services

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"web-ui/internal/models"
)

// ScoreboardService handles scoreboard-related operations
type ScoreboardService struct {
	scoreboards models.ScoreboardMap
}

// NewScoreboardService creates a new scoreboard service
func NewScoreboardService() *ScoreboardService {
	return &ScoreboardService{
		scoreboards: make(models.ScoreboardMap),
	}
}

// LoadScoreboards loads all scoreboards from the filesystem
func (ss *ScoreboardService) LoadScoreboards(challenges models.ChallengeMap) error {
	for id := range challenges {
		challengeDir := filepath.Join("..", "challenge-"+strconv.Itoa(id))
		ss.loadScoreboardForChallenge(id, challengeDir)
	}
	return nil
}

// loadScoreboardForChallenge loads the scoreboard for a specific challenge
func (ss *ScoreboardService) loadScoreboardForChallenge(id int, dir string) {
	scoreboardPath := filepath.Join(dir, "SCOREBOARD.md")
	scoreboardContent, err := ioutil.ReadFile(scoreboardPath)
	if err != nil {
		return
	}

	// Parse scoreboard markdown table
	entries := ss.parseScoreboardMarkdown(string(scoreboardContent), id)
	ss.scoreboards[id] = entries
}

// parseScoreboardMarkdown parses the scoreboard markdown table
func (ss *ScoreboardService) parseScoreboardMarkdown(content string, challengeID int) []models.ScoreboardEntry {
	lines := strings.Split(content, "\n")
	entries := []models.ScoreboardEntry{}

	// Determine format by looking at header line
	var format int = 1 // Default to format 1
	headerLine := ""
	for i, line := range lines {
		if i > 0 && strings.Contains(line, "|") {
			headerLine = line
			break
		}
	}

	if strings.Contains(headerLine, "Rank") && strings.Contains(headerLine, "Username") {
		format = 2
	}

	for i, line := range lines {
		// Skip header and separator lines
		if i < 3 {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip separator lines
		if strings.Contains(line, "---") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		var username string

		if format == 1 {
			// Format is: | Username | Passed Tests | Total Tests |
			username = strings.TrimSpace(parts[1])
		} else {
			// Format is: | Rank | Username | Solution | Date Submitted |
			username = strings.TrimSpace(parts[2])
		}

		// Skip empty usernames or placeholders
		if username == "" || username == "------" || ss.isNumeric(username) {
			continue
		}

		// Use current time for existing entries
		entry := models.ScoreboardEntry{
			Username:    username,
			ChallengeID: challengeID,
			SubmittedAt: time.Now(),
		}

		entries = append(entries, entry)
	}

	return entries
}

// isNumeric checks if a string contains only digits
func (ss *ScoreboardService) isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// GetScoreboard returns the scoreboard for a specific challenge
func (ss *ScoreboardService) GetScoreboard(challengeID int) ([]models.ScoreboardEntry, bool) {
	scoreboard, exists := ss.scoreboards[challengeID]
	return scoreboard, exists
}

// GetAllScoreboards returns all scoreboards
func (ss *ScoreboardService) GetAllScoreboards() models.ScoreboardMap {
	return ss.scoreboards
}

// AddSubmission adds a submission to the scoreboard
func (ss *ScoreboardService) AddSubmission(submission models.Submission) {
	entry := models.ScoreboardEntry{
		Username:    submission.Username,
		ChallengeID: submission.ChallengeID,
		SubmittedAt: submission.SubmittedAt,
	}

	// Add to the scoreboard for this challenge
	if ss.scoreboards[submission.ChallengeID] == nil {
		ss.scoreboards[submission.ChallengeID] = []models.ScoreboardEntry{}
	}

	ss.scoreboards[submission.ChallengeID] = append(ss.scoreboards[submission.ChallengeID], entry)
}
