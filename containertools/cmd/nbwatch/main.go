package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var Version = "development"

func main() {
	log.SetOutput(os.Stderr)
	log.Println("Starting")

	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("nbwatch %s\n", Version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	const contentDir = "/content"

	// NOTE: Watch is non-recursive.
	log.Printf("Watching: %v", contentDir)
	w.Add(contentDir)

	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return fmt.Errorf("reading dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		switch name := e.Name(); name {
		case "data", "model", "artifacts":
		default:
			p := filepath.Join(contentDir, name)
			log.Printf("Watching: %v", p)
			w.Add(p)
		}
	}

	watchLoop(w)

	return nil
}

func watchLoop(w *fsnotify.Watcher) {
	i := int64(0)
	for {
		select {
		// Read from Errors.
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			log.Printf("error: %v", err)
		// Read from Events.
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}

			i++
			path := e.Name

			base := filepath.Base(path)
			// Covers ".git", ".gitignore", ".gitmodules", ".gitattributes", ".ipynb_checkpoints"
			// and also temporary files that Jupyter writes on save like: ".~hello.py"
			if strings.HasPrefix(base, ".") {
				continue
			}

			encoder.Encode(Event{Index: i, Path: path, Op: e.Op.String()})
		}
	}
}

var encoder = json.NewEncoder(os.Stdout)

type Event struct {
	Index int64  `json:"index"`
	Path  string `json:"path"`
	Op    string `json:"op"`
}
