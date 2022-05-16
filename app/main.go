package main

import (
	"bufio"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Usage: your_git.sh run <image> <command> <arg1> <arg2> ...
func main() {
	switch command := os.Args[1]; command {
	case "init":
		handleInit()
	case "cat-file":
		if err := handleCatFile(os.Args[2:]); err != nil {
			fmt.Printf("failed to handle cat-file command: %s\n", err.Error())
			os.Exit(1)
		}
	case "hash-object":
		if err := handleHashObject(os.Args[2:]); err != nil {
			fmt.Printf("failed to handle hash-object command: %s\n", err.Error())
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

	path := filepath.Join(".git", "objects", blobSha[0:2], blobSha[2:])
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open blob file %s: %w", path, err)
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to zlib uncompress blob file: %w", err)
	}

	reader := bufio.NewReader(r)

	header, err := reader.ReadString('\x00')
	if err != nil {
		return fmt.Errorf("failed to read blob header: %w", err)
	}
	header = strings.TrimRight(header, "\x00")

	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read blob file content: %w", err)
	}

	fmt.Print(string(content))
	return nil
}

func handleHashObject(args []string) error {
	if len(args) < 2 {
		return errors.New("insufficient arguments for hash-object command")
	}
	if args[0] != "-w" {
		return errors.New("only supported flag is -w for hash-object command")
	}
	path := args[1]

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	fmt.Println(content)

	h := sha1.New()
	h.Write(content)
	blobSha := hex.EncodeToString(h.Sum(nil))

	data := fmt.Sprintf("blob %d\x00%s", len(blobSha), blobSha)
	objPath := filepath.Join(".git", "objects", string(blobSha[0:2]), string(blobSha[2:]))

	if err = os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return fmt.Errorf("failed to create object dir: %w", err)
	}

	w, err := os.Create(objPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", objPath, err)
	}
	defer w.Close()

	zw := zlib.NewWriter(w)
	if _, err = zw.Write([]byte(data)); err != nil {
		return fmt.Errorf("failed to write blob: %w", err)
	}
	defer zw.Close()

	fmt.Print(blobSha)
	return nil
}
