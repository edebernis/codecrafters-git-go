package main

import (
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Usage: your_git.sh run <image> <command> <arg1> <arg2> ...
func main() {
	switch command := os.Args[1]; command {
	case "init":
		handleInit()
	case "cat-file":
		if err := handleCatFile(os.Args[2:]); err != nil {
			fmt.Println("failed to handle cat-file command: %s", err.Error())
			os.Exit(1)
		}
	default:
		fmt.Println("Unknown command %s", command)
		os.Exit(1)
	}
}

func handleInit() {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.Mkdir(dir, 0755); err != nil {
			fmt.Printf("Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/master\n")
	if err := ioutil.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		fmt.Printf("Error writing file: %s\n", err)
	}

	fmt.Println("Initialized git directory")
}

func handleCatFile(args []string) error {
	if len(args) < 2 {
		return errors.New("insufficient arguments for cat-file command")
	}
	if args[0] != "-p" {
		return errors.New("only supported flag is -p for cat-file command")
	}
	blobSha := args[1]

	path := filepath.Join(".git", "objects", blobSha[0:1], blobSha[2:])
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open blob file %s: %w", path, err)
	}

	r, err := zlib.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to zlib uncompress blob file: %w", err)
	}

	content, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read blob file: %w", err)
	}

	fmt.Println(string(content))
	return nil
}
