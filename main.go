package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

func main() {
	ctx := context.Background()
	execFlag := flag.Bool("exec", false, "Execute the downloaded file")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--exec] s3://bucket/path/to/file\n", os.Args[0])
		os.Exit(1)
	}

	bucket, key, err := parseS3URL(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing S3 URL: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to load SDK config: %v\n", err)
		os.Exit(1)
	}

	client := s3.NewFromConfig(cfg)

	ctxDownload, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := client.GetObject(ctxDownload, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting object from S3: %v\n", err)
		os.Exit(1)
	}
	defer result.Body.Close()

	// Create temp file with original extension if possible
	ext := filepath.Ext(key)
	tmpFile, err := os.CreateTemp("", "s3-download-*"+ext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temporary file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()

	// Make file executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := tmpFile.Chmod(0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error making file executable: %v\n", err)
			os.Exit(1)
		}
	}

	if _, err := io.Copy(tmpFile, result.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying S3 object to file: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	if *execFlag {
		if err := executeFile(tmpPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println(tmpPath)
	}
}
