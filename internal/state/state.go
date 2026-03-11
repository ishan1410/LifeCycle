package state

// Status represents the current phase of the ticket resolution.
type Status string

const (
	StatusOpen                 Status = "OPEN"
	StatusRoutedEcho           Status = "ROUTED_ECHO"
	StatusRoutedReminder       Status = "ROUTED_REMINDER"
	StatusRoutedModifyReminder Status = "ROUTED_MODIFY_REMINDER"
	StatusNeedsMoreInfo        Status = "NEEDS_MORE_INFO"
	StatusResolved             Status = "RESOLVED"
	StatusFailed               Status = "FAILED"
)

// Message represents a single message in the conversation.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// TicketState holds the entire state for the orchestrated execution.
// It acts as the single source of truth for the multi-agent graph.
type TicketState struct {
	TicketID            string
	ConversationHistory []Message
	ExtractedEntities   map[string]interface{}
	Status              Status
	CurrentAgent        string
	ResolutionNotes     string
	RetryCount          int
}

// NewTicketState initializes a new ticket state with a user query.
func NewTicketState(ticketID string, initialQuery string) *TicketState {
	return &TicketState{
		TicketID: ticketID,
		ConversationHistory: []Message{
			{Role: "user", Content: initialQuery},
		},
		ExtractedEntities: make(map[string]interface{}),
		Status:            StatusOpen,
		CurrentAgent:      "Supervisor",
		RetryCount:        0,
	}
}

// AddMessage appends a message to the conversation history.
func (s *TicketState) AddMessage(role, content string) {
	s.ConversationHistory = append(s.ConversationHistory, Message{
		Role:    role,
		Content: content,
	})
}

// UpdateStatus changes the ticket status and resets the retry count if successful.
func (s *TicketState) UpdateStatus(status Status) {
	s.Status = status
}
