package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
)

var Version = "development"

func main() {
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

	w.Add("/content/src")

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
			klog.Error(err)
		// Read from Events.
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}

			i++
			path := e.Name

			// Covers ".git", ".gitignore", ".gitmodules", ".gitattributes", ".ipynb_checkpoints"
			// and also temporary files that Jupyter writes on save like: ".~hello.py"
			if strings.HasPrefix(filepath.Base(path), ".") {
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
