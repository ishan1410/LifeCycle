package agents

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ishanpatel/multi-agent-orchestrator/internal/state"
)

// GraphOrchestrator manages the execution flow of the system.
type GraphOrchestrator struct {
	supervisor     *SupervisorAgent
	echo           *EchoAgent
	reminder       *ReminderAgent
	modifyReminder *ModifyReminderAgent
}

func NewGraphOrchestrator(supervisor *SupervisorAgent, echo *EchoAgent, reminder *ReminderAgent, modifyReminder *ModifyReminderAgent) *GraphOrchestrator {
	return &GraphOrchestrator{
		supervisor:     supervisor,
		echo:           echo,
		reminder:       reminder,
		modifyReminder: modifyReminder,
	}
}

// SetReminderAgent sets or updates the reminder agent after initialization.
// This is used to resolve the cyclic dependency where the Bot needs the Graph,
// and the ReminderAgent needs the Bot.
func (g *GraphOrchestrator) SetReminderAgent(r *ReminderAgent) {
	g.reminder = r
}

// Run executes the graph until a terminal state is reached.
func (g *GraphOrchestrator) Run(ctx context.Context, ticket *state.TicketState) error {
	maxIterations := 10 // Prevent infinite loops
	iterations := 0

	slog.Info("Starting Orchestrator Graph execution", "ticket_id", ticket.TicketID)

	for iterations < maxIterations {
		iterations++
		slog.Info("--- Graph Iteration ---", "iteration", iterations, "current_status", ticket.Status)

		switch ticket.Status {
		case state.StatusOpen:
			ticket.CurrentAgent = "Supervisor"
			if err := g.supervisor.Execute(ctx, ticket); err != nil {
				return fmt.Errorf("supervisor error: %w", err)
			}
		
		case state.StatusRoutedEcho:
			ticket.CurrentAgent = "EchoWorker"
			if err := g.echo.Execute(ctx, ticket); err != nil {
				return fmt.Errorf("echo worker error: %w", err)
			}

		case state.StatusRoutedReminder:
			ticket.CurrentAgent = "ReminderWorker"
			if err := g.reminder.Execute(ctx, ticket); err != nil {
				return fmt.Errorf("reminder worker error: %w", err)
			}
			
		case state.StatusRoutedModifyReminder:
			ticket.CurrentAgent = "ModifyReminderWorker"
			if err := g.modifyReminder.Execute(ctx, ticket); err != nil {
				return fmt.Errorf("modify reminder worker error: %w", err)
			}

		case state.StatusNeedsMoreInfo, state.StatusResolved, state.StatusFailed:
			slog.Info("Reached terminal graph state", "final_status", ticket.Status)
			return nil

		default:
			return fmt.Errorf("unknown state: %s", ticket.Status)
		}
	}

	return fmt.Errorf("graph execution exceeded max iterations (%d)", maxIterations)
}
