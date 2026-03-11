package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/agents"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/bot"
	"github.com/ishanpatel/multi-agent-orchestrator/internal/db"
	"github.com/ishanpatel/multi-agent-orchestrator/pkg/llm"
	"github.com/joho/godotenv"
)

func main() {
	// Configure slog to show text format at Info level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Initializing Telegram Multi-Agent Support Bot")

	// Load .env file
	_ = godotenv.Load()

	if os.Getenv("GEMINI_API_KEY") == "" {
		slog.Error("GEMINI_API_KEY environment variable is required")
		os.Exit(1)
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		slog.Error("TELEGRAM_BOT_TOKEN environment variable is required")
		os.Exit(1)
	}

	adminChatIDStr := os.Getenv("ADMIN_CHAT_ID")
	if adminChatIDStr == "" {
		slog.Error("ADMIN_CHAT_ID environment variable is required")
		os.Exit(1)
	}
	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		slog.Error("Invalid ADMIN_CHAT_ID format", "error", err)
		os.Exit(1)
	}

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
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

	// Initialize Agent Dependencies
	supervisor := agents.NewSupervisorAgent(client)
	echoWorker := agents.NewEchoAgent()
	reminderWorker := agents.NewReminderAgent(client, database)
	modifyReminderWorker := agents.NewModifyReminderAgent(client, database)

	// Combine into the graph orchestrator
	graph := agents.NewGraphOrchestrator(supervisor, echoWorker, reminderWorker, modifyReminderWorker)

	// Initialize the Telegram Bot
	telegramBot, err := bot.NewTelegramBot(botToken, graph)
	if err != nil {
		slog.Error("Failed to initialize Telegram Bot", "error", err)
		os.Exit(1)
	}

	// Initialize and start the Cron background processor
	cron := bot.NewCronProcessor(database, telegramBot, adminChatID)
	go cron.Start(ctx)

	// Run bot polling in a goroutine
	go func() {
		if err := telegramBot.StartPolling(ctx); err != nil {
			slog.Error("Polling stopped", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("Shutting down bot gracefully...")
		cancel() // Cancels the context, stopping the long-polling
		os.Exit(0)
	}()

	// Cloud Run Health Checks: Listen on PORT (Blocking)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("LifeCycle Bot Active"))
	})

	slog.Info("Starting HTTP health server on port " + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("HTTP server failed", "error", err)
	}
}
