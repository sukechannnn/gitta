package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	dbg "runtime/debug"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/giff/config"
	"github.com/sukechannnn/giff/git"
	"github.com/sukechannnn/giff/ui"
)

var version = "dev"

// Application holds the overall application state and configuration
type Application struct {
	App    *tview.Application
	Config *config.AppConfig
}

func main() {
	var debug bool
	var autoRefresh bool
	var showVersion bool
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&autoRefresh, "watch", false, "Watch for file changes and auto-refresh")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.Parse()

	if showVersion {
		if version == "dev" {
			if info, ok := dbg.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}
		}
		fmt.Printf("%s\n", version)
		return
	}

	// Detect the Git repository root
	repoPath, err := git.FindGitRoot(".")
	if err != nil {
		log.Fatalf("Git repository not found: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := tview.NewApplication()

	// Create the application struct
	giffApp := &Application{
		App:    app,
		Config: cfg,
	}

	// SIGINT (Ctrl+c) signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		log.Println("SIGINT received, cleaning up and exiting...")
		// Get PatchFilePath from config and remove it
		if _, err := os.Stat(giffApp.Config.PatchFilePath); err == nil {
			if err := os.Remove(giffApp.Config.PatchFilePath); err != nil {
				log.Printf("Failed to remove %s: %v", giffApp.Config.PatchFilePath, err)
			} else {
				log.Printf("Removed %s", giffApp.Config.PatchFilePath)
			}
		}
		giffApp.App.Stop()
		os.Exit(0)
	}()

	// Application-level key input capture (Ctrl+c workaround)
	giffApp.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			log.Println("Ctrl+c captured at application level, sending SIGINT...")
			gid, err := syscall.Getpgid(os.Getpid())
			if err == nil {
				syscall.Kill(-gid, syscall.SIGINT)
			}
			return nil // Stop propagating the event
		}
		return event // Process other events normally
	})

	// Get files with changes
	stagedFiles, modifiedFiles, untrackedFiles, err := git.GetChangedFiles(repoPath)
	if err != nil {
		log.Fatalf("Failed to get modified files: %v", err)
	}

	// Define file selection handler (defined as a function for recursive use)
	var updateFileList func()

	updateFileList = func() {
		updatedStagedFiles, updatedModifiedFiles, updatedUntrackedFiles, err := git.GetChangedFiles(repoPath)
		if err != nil {
			log.Fatalf("Failed to get updated files: %v", err)
		}
		rootEditor := ui.RootEditor(
			giffApp.App,
			updatedStagedFiles,
			updatedModifiedFiles,
			updatedUntrackedFiles,
			repoPath,
			giffApp.Config.PatchFilePath,
			updateFileList,
			autoRefresh,
		)
		giffApp.App.SetRoot(rootEditor, true)
	}

	// Create the initial view (file list) and set it as root
	// The onSelect parameter is currently unused, so nil is passed
	initialView := ui.RootEditor(giffApp.App, stagedFiles, modifiedFiles, untrackedFiles, repoPath, giffApp.Config.PatchFilePath, updateFileList, autoRefresh)
	giffApp.App.SetRoot(initialView, true)

	// Run the application only once in main
	if err := giffApp.App.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
