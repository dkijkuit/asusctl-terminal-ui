package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	term := NewTerminal()
	backend := NewBackend()

	if err := term.EnterRaw(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enter raw mode: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're running this in a terminal.\n")
		os.Exit(1)
	}

	// Ensure cleanup on any exit
	defer term.ExitRaw()

	// Handle SIGINT/SIGTERM gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		term.ExitRaw()
		os.Exit(0)
	}()

	// Handle SIGWINCH (terminal resize)
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)

	app := NewApp(term, backend)
	app.Init()

	// Initial render
	app.Render()

	// Main event loop
	for app.running {
		// Check for resize signal (non-blocking)
		select {
		case <-winchCh:
			term.updateSize()
			app.Render()
			continue
		default:
		}

		// Read key (with timeout from raw mode settings)
		key := ReadKey()
		if key.Type == KeyChar && key.Char == 0 {
			// Timeout — re-render to update status messages, etc.
			app.Render()
			continue
		}

		app.HandleKey(key)
		if app.running {
			app.Render()
		}
	}
}
