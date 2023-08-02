package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
)

func main() {
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

	go watchLoop(w)
	<-make(chan struct{}) // Block forever

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

			// Just print the event nicely aligned, and keep track how many
			// events we've seen.
			i++

			path := e.Name
			//path, err := filepath.EvalSymlinks(e.Name)
			//if err != nil {
			//	klog.Error(err)
			//	continue
			//}

			switch filepath.Base(path) {
			case ".git", ".gitignore", ".gitmodules", ".gitattributes", ".ipynb_checkpoints":
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
