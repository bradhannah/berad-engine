// Command berad-play plays a project in a window of its own (increment 62):
// the engine's player and nothing else. A game's own program is the same
// player with the game's own code beside it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bradhannah/berad-engine/engine/player"
)

// scriptEnv names a JSON file of steps that play the game by itself in
// place of the keyboard, for tests that open the real window.
const scriptEnv = "BERAD_PLAY_SCRIPT"

// errUsage reports a command line that names no project.
var errUsage = errors.New("usage: berad-play --project DIR [--seed N]")

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		report(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// report writes a message for the person running the game. If standard
// error itself fails there is nowhere left to report to.
func report(stderr io.Writer, message string) {
	if _, err := io.WriteString(stderr, "berad-play: "+message+"\n"); err != nil {
		return
	}
}

func run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("berad-play", flag.ContinueOnError)
	flags.SetOutput(stderr)
	project := flags.String("project", "", "the project folder to play; it is only read")
	seed := flags.Uint64("seed", 0, "fix the NPCs' random choices so a run repeats; 0 picks a new seed each run")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	if *project == "" || flags.NArg() > 0 {
		return errUsage
	}
	info, err := os.Stat(*project)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s is not a folder", errUsage, *project)
	}
	options := player.Options{
		Project: os.DirFS(*project), Title: title(*project), Seed: *seed,
		Report: func(problem string) { report(stderr, problem) }, Script: nil,
	}
	if options.Seed == 0 {
		options.Seed = uint64(time.Now().UnixNano())
	}
	if path := os.Getenv(scriptEnv); path != "" {
		if options.Script, err = player.ReadScript(path); err != nil {
			return err //nolint:wrapcheck // The player's own words.
		}
	}

	return player.Run(options) //nolint:wrapcheck // The player's own words.
}

// title is the window's title: the folder that holds the project, or the
// project folder's own name.
func title(project string) string {
	absolute, err := filepath.Abs(project)
	if err != nil {
		return filepath.Base(project)
	}
	if name := filepath.Base(absolute); name != "project" {
		return name
	}

	return filepath.Base(filepath.Dir(absolute))
}
