package services

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"web-ui/internal/models"
)

// ExecutionService handles code execution and testing
type ExecutionService struct{}

// NewExecutionService creates a new execution service
func NewExecutionService() *ExecutionService {
	return &ExecutionService{}
}

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	Passed      bool   `json:"passed"`
	Output      string `json:"output"`
	ExecutionMs int64  `json:"executionMs"`
}

// RunCode executes the provided code against a challenge's tests
func (es *ExecutionService) RunCode(code string, challenge *models.Challenge) ExecutionResult {
	start := time.Now()

	// Create temporary directory for execution
	tempDir, err := ioutil.TempDir("", "challenge-exec")
	if err != nil {
		return ExecutionResult{
			Passed: false,
			Output: fmt.Sprintf("Failed to create temporary directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	// Write the submitted code to temporary file
	codePath := filepath.Join(tempDir, "solution-template.go")
	err = ioutil.WriteFile(codePath, []byte(code), 0644)
	if err != nil {
		return ExecutionResult{
			Passed: false,
			Output: fmt.Sprintf("Failed to write code file: %v", err),
		}
	}

	// Write the test file to temporary directory
	testPath := filepath.Join(tempDir, "solution_test.go")
	err = ioutil.WriteFile(testPath, []byte(challenge.TestFile), 0644)
	if err != nil {
		return ExecutionResult{
			Passed: false,
			Output: fmt.Sprintf("Failed to write test file: %v", err),
		}
	}

	// Initialize Go module
	err = es.initGoModule(tempDir, challenge.ID)
	if err != nil {
		return ExecutionResult{
			Passed: false,
			Output: fmt.Sprintf("Failed to initialize Go module: %v", err),
		}
	}

	// Automatically detect and install dependencies based on imports
	err = es.installDependencies(tempDir, code, challenge.ID)
	if err != nil {
		return ExecutionResult{
			Passed: false,
			Output: fmt.Sprintf("Failed to install dependencies: %v", err),
		}
	}

	// Run tests
	cmd := exec.Command("go", "test", "-v")
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	executionTime := time.Since(start).Milliseconds()
	outputStr := string(output)

	result := ExecutionResult{
		Output:      outputStr,
		ExecutionMs: executionTime,
	}

	if err == nil {
		result.Passed = true
	} else {
		// Check if tests ran but failed (this is the key logic!)
		if _, ok := err.(*exec.ExitError); ok {
			// Test ran but failed - this means tests executed but some failed
			result.Passed = false // Tests failed, so Passed = false
		} else {
			// Command couldn't be run - this is a real error
			result.Passed = false
			result.Output = fmt.Sprintf("Failed to run tests: %v\n%s", err, outputStr)
		}
	}

	return result
}

// initGoModule initializes a Go module in the temporary directory
func (es *ExecutionService) initGoModule(tempDir string, challengeID int) error {
	// Initialize go.mod
	cmd := exec.Command("go", "mod", "init", fmt.Sprintf("challenge-%d", challengeID))
	cmd.Dir = tempDir
	return cmd.Run()
}

// installDependencies installs dependencies for the given challenge
func (es *ExecutionService) installDependencies(tempDir string, code string, challengeID int) error {
	// Detect imports from the code
	requiredPackages := es.detectRequiredPackages(code, challengeID)

	if len(requiredPackages) == 0 {
		return nil // No external dependencies needed
	}

	// Install each required package
	for _, pkg := range requiredPackages {
		fmt.Printf("Installing dependency: %s\n", pkg)
		cmd := exec.Command("go", "get", pkg)
		cmd.Dir = tempDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install package %s: %v\nOutput: %s", pkg, err, string(output))
		}
	}

	// Run go mod tidy to clean up dependencies
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tempDir
	tidyCmd.Run() // Ignore errors for tidy

	return nil
}

// detectRequiredPackages analyzes the code to detect required external packages
func (es *ExecutionService) detectRequiredPackages(code string, challengeID int) []string {
	packages := make(map[string]bool)

	// Common package mappings for known dependencies
	knownPackages := map[string][]string{
		"github.com/mattn/go-sqlite3": {"github.com/mattn/go-sqlite3"},
		"database/sql":                {"github.com/mattn/go-sqlite3"}, // SQLite driver needed for database/sql
		"google.golang.org/grpc":      {"google.golang.org/grpc", "google.golang.org/grpc/codes", "google.golang.org/grpc/status"},
		"github.com/google/uuid":      {"github.com/google/uuid"},
		"github.com/gorilla/mux":      {"github.com/gorilla/mux"},
		"github.com/gin-gonic/gin":    {"github.com/gin-gonic/gin"},
		"github.com/stretchr/testify": {"github.com/stretchr/testify"},
		"go.mongodb.org/mongo-driver": {"go.mongodb.org/mongo-driver/mongo", "go.mongodb.org/mongo-driver/bson"},
		"github.com/redis/go-redis":   {"github.com/redis/go-redis/v9"},
		"gorm.io/gorm":                {"gorm.io/gorm", "gorm.io/driver/sqlite"},
	}

	// Challenge-specific package requirements
	challengePackages := map[int][]string{
		13: {"github.com/mattn/go-sqlite3"},                                                             // SQL Database Operations
		14: {"google.golang.org/grpc", "google.golang.org/grpc/codes", "google.golang.org/grpc/status"}, // gRPC
		9:  {"github.com/google/uuid"},                                                                  // RESTful API with UUID
	}

	// Add challenge-specific packages
	if challengePkgs, exists := challengePackages[challengeID]; exists {
		for _, pkg := range challengePkgs {
			packages[pkg] = true
		}
	}

	// Scan code for import statements
	lines := strings.Split(code, "\n")
	inImportBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Handle import blocks
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Handle single import statements
		if strings.HasPrefix(line, "import ") || inImportBlock {
			// Extract import path
			importPath := es.extractImportPath(line)
			if importPath != "" {
				// Check if this import requires external packages
				if deps, exists := knownPackages[importPath]; exists {
					for _, dep := range deps {
						packages[dep] = true
					}
				} else if es.isExternalPackage(importPath) {
					packages[importPath] = true
				}
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(packages))
	for pkg := range packages {
		result = append(result, pkg)
	}

	return result
}

// extractImportPath extracts the import path from an import line
func (es *ExecutionService) extractImportPath(line string) string {
	// Remove 'import' keyword
	line = strings.TrimPrefix(line, "import")
	line = strings.TrimSpace(line)

	// Remove quotes and comments
	if strings.Contains(line, "\"") {
		parts := strings.Split(line, "\"")
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	return ""
}

// isExternalPackage determines if an import path is an external package
func (es *ExecutionService) isExternalPackage(importPath string) bool {
	// Standard library packages (don't need go get)
	standardLibs := map[string]bool{
		"fmt": true, "strings": true, "strconv": true, "time": true,
		"os": true, "io": true, "bufio": true, "bytes": true,
		"encoding/json": true, "encoding/xml": true, "encoding/csv": true,
		"net/http": true, "net/url": true, "net": true,
		"database/sql": true, "context": true, "sync": true,
		"regexp": true, "sort": true, "math": true, "crypto": true,
		"log": true, "errors": true, "flag": true, "path": true,
		"path/filepath": true, "testing": true, "reflect": true,
	}

	// Check if it's a standard library package
	if standardLibs[importPath] {
		return false
	}

	// Check if it starts with standard library prefixes
	standardPrefixes := []string{
		"crypto/", "encoding/", "net/", "os/", "path/",
		"text/", "html/", "image/", "go/", "runtime/",
		"unicode/", "mime/", "archive/", "compress/",
		"container/", "debug/", "index/", "internal/",
	}

	for _, prefix := range standardPrefixes {
		if strings.HasPrefix(importPath, prefix) {
			return false
		}
	}

	// If it contains a dot, it's likely an external package
	return strings.Contains(importPath, ".")
}

// SaveSubmissionRequest represents a request to save a submission to filesystem
type SaveSubmissionRequest struct {
	Username    string `json:"username"`
	ChallengeID int    `json:"challengeId"`
	Code        string `json:"code"`
}

// SaveSubmissionResponse represents the response from saving a submission
type SaveSubmissionResponse struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	FilePath    string   `json:"filePath"`
	GitCommands []string `json:"gitCommands"`
}

// SaveSubmissionToFilesystem saves a user's submission to the filesystem
func (es *ExecutionService) SaveSubmissionToFilesystem(request SaveSubmissionRequest) SaveSubmissionResponse {
	// Get working directory for correct relative paths
	workDir, _ := os.Getwd()

	// Try different path approaches to handle potential path issues
	var submissionDir string
	var fileSaved bool

	// Try multiple path options to ensure it works in different environments
	pathOptions := []string{
		// Option 1: From web-ui directory (standard case)
		filepath.Join("..", fmt.Sprintf("challenge-%d", request.ChallengeID), "submissions", request.Username),
		// Option 2: From root workspace
		filepath.Join(fmt.Sprintf("challenge-%d", request.ChallengeID), "submissions", request.Username),
		// Option 3: Absolute path from detected workspace root
		filepath.Join(workDir, "..", fmt.Sprintf("challenge-%d", request.ChallengeID), "submissions", request.Username),
	}

	for _, dirPath := range pathOptions {
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			continue
		}

		solutionFile := filepath.Join(dirPath, "solution-template.go")
		err = ioutil.WriteFile(solutionFile, []byte(request.Code), 0644)
		if err != nil {
			continue
		}

		submissionDir = dirPath
		fileSaved = true
		break
	}

	if !fileSaved {
		return SaveSubmissionResponse{
			Success: false,
			Message: "Failed to save solution to any available path",
		}
	}

	// Return success response with git commands
	return SaveSubmissionResponse{
		Success:  true,
		Message:  "Solution saved to filesystem",
		FilePath: filepath.Join(submissionDir, "solution-template.go"),
		GitCommands: []string{
			"cd " + filepath.Join(workDir, ".."),
			fmt.Sprintf("git add %s", filepath.Join(fmt.Sprintf("challenge-%d", request.ChallengeID), "submissions", request.Username, "solution-template.go")),
			fmt.Sprintf("git commit -m \"Add solution for Challenge %d\"", request.ChallengeID),
			"git push origin main",
		},
	}
}
