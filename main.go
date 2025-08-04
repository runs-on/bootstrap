package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"bootstrap/internal/s3client"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func parseS3URL(s3URL string) (bucket, key string, err error) {
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", "", fmt.Errorf("invalid S3 URL: %w", err)
	}
	if u.Scheme != "s3" {
		return "", "", fmt.Errorf("not an S3 URL (should start with s3://)")
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

func executeFile(path string) error {
	cmd := &exec.Cmd{}

	if runtime.GOOS == "windows" {
		// On Windows, try to detect if it's a script that needs an interpreter
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".bat", ".cmd":
			cmd = exec.Command("cmd", "/C", path)
		case ".ps1":
			cmd = exec.Command("powershell", "-File", path)
		case ".py":
			cmd = exec.Command("python", path)
		default:
			// For .exe and other executables
			cmd = exec.Command(path)
		}
	} else {
		// On Unix systems, execute directly
		cmd = exec.Command(path)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shutdownSystem(duration time.Duration, debug bool) error {
	var cmd *exec.Cmd
	time.Sleep(duration)
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("shutdown", "/s", "/t", "0")
	default: // Linux and others
		cmd = exec.Command("sudo", "shutdown", "-h", "now")
	}
	if debug {
		fmt.Printf("Debug: Would execute command: %v\n", cmd.Args)
		return nil
	}
	return cmd.Run()
}

func main() {
	ctx := context.Background()

	// Create cancellable context for signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create temp file path holder
	var tmpPath string

	// Goroutine for signal handling
	go func() {
		<-sigChan
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
		cancel()
		os.Exit(1)
	}()

	saveFlag := flag.String("save", "", "Save the downloaded file to the specified path instead of a temporary location")
	execFlag := flag.Bool("exec", false, "Execute the downloaded file")
	postExecFlag := flag.String("post-exec", "", "Action to take after execution (only used with --exec). Valid values: shutdown")
	debugFlag := flag.Bool("debug", false, "Debug mode - skips post-exec actions")
	flag.Parse()

	if *postExecFlag != "" && !*execFlag {
		fmt.Fprintf(os.Stderr, "Error: --post-exec can only be used with --exec\n")
		os.Exit(1)
	}

	if *postExecFlag != "" && *postExecFlag != "shutdown" {
		fmt.Fprintf(os.Stderr, "Error: invalid --post-exec value. Valid values: shutdown\n")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--exec] [--save path] s3://bucket/path/to/file\n", os.Args[0])
		os.Exit(1)
	}

	bucket, key, err := parseS3URL(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing S3 URL: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to load AWS config: %v\n", err)
		os.Exit(1)
	}

	client := s3.NewFromConfig(cfg)

	ctxDownload, cancelDownload := context.WithTimeout(ctx, 30*time.Second)
	defer cancelDownload()

	result, err := s3client.Download(ctxDownload, client, bucket, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer result.Close()

	var targetPath string
	var targetFile *os.File

	if *saveFlag != "" {
		// Use the specified save path
		targetPath = *saveFlag

		// Create parent directories if they don't exist
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
			os.Exit(1)
		}

		// Create the target file
		targetFile, err = os.Create(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Create temp file with original extension if possible
		targetFile, err = os.CreateTemp("", "bootstrap-*-"+filepath.Base(key))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating temporary file: %v\n", err)
			os.Exit(1)
		}
		targetPath = targetFile.Name()
		tmpPath = targetPath // Set tmpPath for cleanup handling

		// Ensure cleanup of file if exec flag is set
		if *execFlag {
			defer os.Remove(targetPath)
		}
	}

	if _, err := io.Copy(targetFile, result); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying S3 object to file: %v\n", err)
		os.Exit(1)
	}

	// Make file executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := targetFile.Chmod(0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error making file executable: %v\n", err)
			os.Exit(1)
		}
	}

	targetFile.Close()

	if *execFlag {
		var exitStatus int
		if err := executeFile(targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing file: %v\n", err)
			exitStatus = 1
		}

		if *postExecFlag == "shutdown" {
			fmt.Println("System will shutdown in 20 seconds...")
			if err := shutdownSystem(time.Duration(20)*time.Second, *debugFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error initiating shutdown: %v\n", err)
				exitStatus = 1
			}
		}

		os.Exit(exitStatus)
	} else {
		fmt.Println(targetPath)
	}
}
