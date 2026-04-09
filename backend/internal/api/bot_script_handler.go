package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// scriptBaseDir is the root directory where uploaded scripts are stored.
// Each script is saved as <scriptBaseDir>/<scriptID>.py so the path can
// be resolved server-side from the ID alone — never exposed to the client.
const scriptBaseDir = "/tmp/synthbull_scripts"

// BotScriptHandler handles API requests for managing bot scripts.
type BotScriptHandler struct {
	// In a real implementation, this would have a service to store/retrieve scripts.
}

// NewBotScriptHandler creates a new handler for bot script operations.
func NewBotScriptHandler() *BotScriptHandler {
	return &BotScriptHandler{}
}

// UploadCustomScript handles the upload and initial validation of a custom bot script.
func (h *BotScriptHandler) UploadCustomScript(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	script := c.PostForm("script")
	if script == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Script content cannot be empty"})
		return
	}

	// Validate script for forbidden imports and keywords
	if err := ValidateScriptASTSafety(script); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Script validation failed: " + err.Error()})
		return
	}

	// Save the script to a fixed directory keyed by script_id.
	// The path is resolved server-side and never returned to the client.
	if err := os.MkdirAll(scriptBaseDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create script directory"})
		return
	}

	scriptID := uuid.New().String()
	scriptPath := filepath.Join(scriptBaseDir, scriptID+".py")

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save script"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Script uploaded and validated successfully.",
		"script_id": scriptID,
	})
}

// ResolveScriptPath returns the filesystem path for a given script ID.
// Returns an error if the ID is not a valid UUID or the file does not exist.
func ResolveScriptPath(scriptID string) (string, error) {
	// Validate that scriptID looks like a UUID to prevent path traversal.
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	if !uuidPattern.MatchString(scriptID) {
		return "", fmt.Errorf("invalid script ID format")
	}
	p := filepath.Join(scriptBaseDir, scriptID+".py")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("script not found: %s", scriptID)
	}
	return p, nil
}

// ValidateScriptASTSafety performs a basic static analysis of the script to
// prevent the use of dangerous modules or functions.
//
// IMPORTANT: This is a defence-in-depth heuristic, NOT a real sandbox.
// The actual security boundary is the Docker container (no network, resource
// limits, read-only FS, non-root user). This check only catches obvious
// mistakes early so the user gets a fast, friendly error message.
func ValidateScriptASTSafety(script string) error {
	// Forbidden import patterns — matched as substrings because they are
	// multi-word and unlikely to appear as legitimate identifiers.
	forbiddenImports := []string{
		"import os", "from os",
		"import sys", "from sys",
		"import subprocess", "from subprocess",
		"import socket", "from socket",
		"import multiprocessing",
		"import threading",
		"import ctypes",
		"import _thread",
		"import importlib", "from importlib",
		"import shutil", "from shutil",
		"import signal", "from signal",
		"import tempfile", "from tempfile",
	}

	for _, pattern := range forbiddenImports {
		if strings.Contains(script, pattern) {
			return fmt.Errorf("forbidden import found: %q", pattern)
		}
	}

	// Forbidden built-in function calls — matched with word-boundary regex
	// so "open(" is caught but "portfolio_open" or "is_open" are not.
	forbiddenBuiltins := []string{
		`\bopen\s*\(`,
		`\beval\s*\(`,
		`\bexec\s*\(`,
		`\bcompile\s*\(`,
		`\bglobals\s*\(`,
		`\blocals\s*\(`,
		`\bvars\s*\(`,
		`\bbreakpoint\s*\(`,
		`\bgetattr\s*\(`,
		`\bsetattr\s*\(`,
		`\bdelattr\s*\(`,
		`__import__`,
		`__builtins__`,
		`__subclasses__`,
	}

	for _, pattern := range forbiddenBuiltins {
		re := regexp.MustCompile(pattern)
		if re.MatchString(script) {
			return fmt.Errorf("forbidden builtin or keyword found: %q", pattern)
		}
	}

	return nil
}
