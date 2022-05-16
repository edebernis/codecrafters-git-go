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
	case "ls-tree":
		if err := handleLsTree(os.Args[2:]); err != nil {
			fmt.Printf("failed to handle ls-tree command: %s\n", err.Error())
			os.Exit(1)
		}
	case "write-tree":
		if err := handleWriteTree(os.Args[2:]); err != nil {
			fmt.Printf("failed to handle write-tree command: %s\n", err.Error())
			os.Exit(1)
		}
	case "commit-tree":
		if err := handleCommitTree(os.Args[2:]); err != nil {
			fmt.Printf("failed to handle commit-tree command: %s\n", err.Error())
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

	blobSha, err := hashBlob(path)
	if err != nil {
		return fmt.Errorf("failed to hash object: %w", err)
	}

	fmt.Print(blobSha)
	return nil
}

func handleLsTree(args []string) error {
	if len(args) < 2 {
		return errors.New("insufficient arguments for ls-tree command")
	}
	if args[0] != "--name-only" {
		return errors.New("only supported flag is --name-only for ls-tree command")
	}
	treeSha := args[1]

	path := filepath.Join(".git", "objects", treeSha[0:2], treeSha[2:])
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open tree file %s: %w", path, err)
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to zlib uncompress tree file: %w", err)
	}

	reader := bufio.NewReader(r)

	header, err := reader.ReadString('\x00')
	if err != nil {
		return fmt.Errorf("failed to read tree header: %w", err)
	}
	header = strings.TrimRight(header, "\x00")

	for {
		entry, err := reader.ReadString('\x00')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read entry: %w", err)
		}
		entry = strings.TrimRight(entry, "\x00")

		splitEntry := strings.Split(entry, " ")
		fmt.Println(splitEntry[1])

		if _, err := io.ReadFull(reader, make([]byte, 20)); err != nil {
			return fmt.Errorf("failed to read entry sha: %w", err)
		}
	}

	return nil
}

func handleWriteTree(args []string) error {
	treeSha, err := hashTree(".")
	if err != nil {
		return fmt.Errorf("failed to hash working directory tree: %w", err)
	}

	fmt.Print(hex.EncodeToString(treeSha))
	return nil
}

func hashTree(root string) ([]byte, error) {
	var data string

	files, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read working directory: %w", err)
	}

	for _, f := range files {
		if f.Name() == ".git" {
			continue
		}
		path := filepath.Join(root, f.Name())
		if f.IsDir() {
			treeSha, err := hashTree(path)
			if err != nil {
				return nil, fmt.Errorf("failed to hash tree: %w", err)
			}
			data += fmt.Sprintf("40000 %s\x00%s", f.Name(), string(treeSha))
			continue
		}
		blobSha, err := hashBlob(path)
		if err != nil {
			return nil, fmt.Errorf("failed to hash blob: %w", err)
		}
		data += fmt.Sprintf("100644 %s\x00%s", f.Name(), blobSha)
	}

	content := fmt.Sprintf("tree %d\x00%s", len(data), data)

	h := sha1.New()
	h.Write([]byte(content))
	treeSha := h.Sum(nil)
	treeShaHex := hex.EncodeToString(treeSha)

	if err := writeObject(treeShaHex, content); err != nil {
		return nil, fmt.Errorf("failed to write tree object: %w", err)
	}

	return treeSha, nil
}

func hashBlob(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	data := fmt.Sprintf("blob %d\x00%s", len(content), content)

	h := sha1.New()
	h.Write([]byte(data))
	blobSha := hex.EncodeToString(h.Sum(nil))

	if err := writeObject(blobSha, data); err != nil {
		return "", fmt.Errorf("failed to write blob object: %w", err)
	}

	return blobSha, nil
}

func writeObject(sha, data string) error {
	objPath := filepath.Join(".git", "objects", string(sha[0:2]), string(sha[2:]))

	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
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

	return nil
}

func handleCommitTree(args []string) error {
	if len(args) < 5 {
		return errors.New("insufficient arguments for commit-tree command")
	}
	treeSha := args[0]
	parentCommitSha := args[2]
	message := args[4]

	content := "tree " + treeSha + `
parent ` + parentCommitSha + `
author Emeric de Bernis <emeric.debernis@qonto.com> 1652700438 +0200
committer Emeric de Bernis <emeric.debernis@qonto.com> 1652700438 +0200

` + message

	data := fmt.Sprintf("commit %d\x00%s", len(content), content)

	h := sha1.New()
	h.Write([]byte(data))
	commitSha := hex.EncodeToString(h.Sum(nil))

	if err := writeObject(commitSha, data); err != nil {
		return fmt.Errorf("failed to write commit object: %w", err)
	}

	return nil
}
