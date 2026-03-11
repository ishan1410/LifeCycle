package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/agents"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/db"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
	"github.com/ishanpatel/multi-agent-orchestrator/pkg/llm"
	"github.com/joho/godotenv"
)

func main() {
	// Configure slog to show text format at Info level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Initializing Autonomous Multi-Agent Support Orchestrator")

	// Load .env file if it exists
	_ = godotenv.Load()

	if os.Getenv("GEMINI_API_KEY") == "" {
		slog.Error("GEMINI_API_KEY environment variable is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Initialize the LLM client wrapper
	client, err := llm.NewClient(ctx)
	if err != nil {
		slog.Error("Failed to initialize LLM client", "error", err)
		os.Exit(1)
	}

	// Initialize Database in the cloud
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		slog.Error("DATABASE_URL must be set")
		os.Exit(1)
	}
	database, err := db.NewDatabase(dbUrl)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initialize the various agents
	supervisor := agents.NewSupervisorAgent(client)
	echoWorker := agents.NewEchoAgent()
	reminderWorker := agents.NewReminderAgent(client, database)
	modifyReminderWorker := agents.NewModifyReminderAgent(client, database)

	// Combine into the graph orchestrator
	graph := agents.NewGraphOrchestrator(supervisor, echoWorker, reminderWorker, modifyReminderWorker)

	// Create a mock ticket simulating the user's issue
	// We include the IDs so the autonomous agents have enough context to resolve it without pausing to ask the user.
	initialQuery := "I was charged twice and my server is down. My ticket ID is TKT-90210 and my server ID is SVR-404."
	ticket := state.NewTicketState("TKT-90210", initialQuery)

	slog.Info("Simulating user request", "query", initialQuery)

	// Execute the multi-agent graph
	if err := graph.Run(ctx, ticket); err != nil {
		slog.Error("Graph execution failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Execution Complete",
		"final_status", ticket.Status,
		"resolution_notes", ticket.ResolutionNotes,
		"agent", ticket.CurrentAgent,
	)

	// Print conversation history
	fmt.Println("\n--- Final Conversation History ---")
	for _, msg := range ticket.ConversationHistory {
		fmt.Printf("[%s]: %s\n", msg.Role, msg.Content)
	}
}
