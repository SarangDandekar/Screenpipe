package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/your-username/meeting-coach/detector"
	"github.com/your-username/meeting-coach/logs"
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	// Check for --view-logs or -logs flag
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "--view-logs" || arg == "-logs" || arg == "--logs" {
			fmt.Println()
			fmt.Println("  ==========================================")
			fmt.Println("    AI MEETING COACH — ACTIVITY LOGS")
			fmt.Println("  ==========================================")
			logs.PrintAllLogs()
			os.Exit(0)
		}
	}

	fmt.Println()
	fmt.Println("  ==========================================")
	fmt.Println("    AI MEETING COACH")
	fmt.Println("    Powered by Screenpipe + Network Monitor")
	fmt.Println("  ==========================================")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down Meeting Coach...")
		os.Exit(0)
	}()

	md := detector.NewMeetingDetector()
	md.Start()
}
