package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
)

// editorDocuments holds a project folder's documents as an editor window
// holds them: each decoded by the plug-in of its format, and the project
// index listing them all.
func editorDocuments(t *testing.T, projectDir string) *frametest.Docs {
	t.Helper()
	registry, err := plugins()
	if err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	var entries []project.Entry
	err = filepath.WalkDir(projectDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(projectDir, path)
		if entry.IsDir() {
			if relative == ".berad" || relative == "audio" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".json" || strings.HasSuffix(path, ".layout.json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		envelope, err := content.Decode(raw)
		if err != nil {
			return err
		}
		kind, known := registry.Document(envelope.Format)
		if !known {
			t.Fatalf("%s: the editor has no plug-in for format %q", relative, envelope.Format)
		}
		document, err := kind.Decode(envelope.Data)
		if err != nil {
			return err
		}
		docs.Put(envelope.ID, envelope.Name, document)
		entries = append(entries, project.Entry{ID: envelope.ID, Name: envelope.Name, Format: envelope.Format})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	docs.Put(project.IndexID(), "Project", project.NewIndex(entries))
	return docs
}

// TestTheSamplesFolderIsWhatTheEditorHolds makes the sample as the program
// does without a window, then gathers its game twice: from the files in its folder, as a
// game reads them, and from the documents an editor window holds, as the Play
// tab reads them. They are the same game (increment 62), and it starts.
func TestTheSamplesFolderIsWhatTheEditorHolds(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	var said strings.Builder
	if err := runOpen([]string{"--project", projectDir, "--sample", "--no-window", "--exit-after", "1s"}, &said); err != nil {
		t.Fatalf("open: %v\n%s", err, said.String())
	}
	folder, err := gameload.ReadFolder(os.DirFS(projectDir))
	if err != nil {
		t.Fatal(err)
	}
	if unread := folder.Unread(); len(unread) != 0 {
		t.Fatalf("every document of the sample reads: %v", unread)
	}
	fromFolder, ready := gameload.Load(folder, nil)
	if !ready {
		t.Fatal("a folder never waits")
	}
	fromEditor, ready := gameload.Load(docsource.New(editorDocuments(t, projectDir)), nil)
	if !ready {
		t.Fatal("the editor holds every document")
	}
	// Each names things from its own source; what they name is compared by
	// the values and the places gathered.
	fromFolder.Project.Names, fromEditor.Project.Names = nil, nil
	if !reflect.DeepEqual(fromFolder, fromEditor) {
		t.Fatalf("the folder and the editor gather different games:\nfolder: %+v\neditor: %+v", fromFolder, fromEditor)
	}
	if fromFolder.Project.Settings.Start.IsZero() || len(fromFolder.Project.Floors) != 2 ||
		len(fromFolder.Project.NPCs) == 0 || len(fromFolder.Project.Workflows) == 0 || fromFolder.Project.Values == nil {
		t.Fatalf("the sample has a start, two floors, its clerk, workflows and values: %+v", fromFolder.Project)
	}
	for _, shown := range []string{art.Player, "wall.a"} {
		if _, has := fromFolder.Tiles[shown]; !has {
			t.Fatalf("%s shows a sprite: %v", shown, fromFolder.Tiles)
		}
	}
	if len(fromFolder.Cues) == 0 {
		t.Fatal("the clerk's day plays a cue")
	}
	if _, err := game.New(fromFolder.Project); err != nil {
		t.Fatalf("the sample starts from its folder: %v", err)
	}
}
