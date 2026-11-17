package engine_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// What only the player and the programs may use, so everything else in the
// engine stays headless and a run repeats exactly (increment 62): Ebitengine,
// berad-audio's Ebitengine backend, and the player itself. Nothing in the
// engine may use the editor.
const (
	ebitengine   = "github.com/hajimehoshi/ebiten"
	audioBackend = "github.com/bradhannah/berad-sdk/modules/berad-audio/backend"
	playerPath   = "github.com/bradhannah/berad-engine/engine/player"
	editorPath   = "github.com/bradhannah/berad-engine/editor"
)

// TestOnlyThePlayerUsesEbitengine reads every Go file of the engine module.
func TestOnlyThePlayerUsesEbitengine(t *testing.T) {
	t.Parallel()
	files := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return walkErr
		}
		files++
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(path)
		mayDraw := strings.HasPrefix(slashed, "player/") || strings.HasPrefix(slashed, "cmd/")
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			switch {
			case imported == editorPath || strings.HasPrefix(imported, editorPath+"/"):
				t.Errorf("%s uses the editor (%s); the engine never does", slashed, imported)
			case mayDraw:
			case strings.HasPrefix(imported, ebitengine), strings.HasPrefix(imported, audioBackend),
				imported == playerPath, strings.HasPrefix(imported, playerPath+"/"):
				t.Errorf("%s uses %s; only the player and the programs may", slashed, imported)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 50 {
		t.Fatalf("the engine's files were not found: %d", files)
	}
}
