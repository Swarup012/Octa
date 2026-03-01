// Octa - Personal AI Agent
// License: MIT

package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/Swarup012/solo/pkg/bus"
	"github.com/Swarup012/solo/pkg/logger"
)

// Publisher is the subset of *bus.MessageBus used by Dispatcher.
// Defined as an interface so it is mockable in tests.
type Publisher interface {
	PublishOutbound(ctx context.Context, msg bus.OutboundMessage) error
}

// Ensure *bus.MessageBus satisfies Publisher at compile time.
var _ Publisher = (*bus.MessageBus)(nil)

// CalendarEvent represents a calendar event for reminder checking.
type CalendarEvent struct {
	ID       string
	Title    string
	StartRFC string // RFC3339 format
}

// CalendarUpcomingLister fetches upcoming calendar events within a time window.
type CalendarUpcomingLister func(ctx context.Context, window time.Duration) ([]CalendarEvent, error)

// EmailSender sends an email immediately.
type EmailSender func(ctx context.Context, to []string, subject, body string, isHTML bool, cc []string) error

// StateManager tracks which events have already triggered notifications.
type StateManager interface {
	IsEventNotified(id string) bool
	MarkEventNotified(id string) error
}

// Dispatcher holds the logic for dispatching scheduled emails and sending
// meeting reminders. It owns no goroutines or tickers — those are driven
// by the shared pkg/scheduler.Scheduler registered in cmd_gateway.
type Dispatcher struct {
	queue        *EmailQueue
	sender       EmailSender
	calLister    CalendarUpcomingLister
	msgBus       Publisher
	stateManager StateManager
}

// NewDispatcher creates a Dispatcher.
// calLister and stateManager may be nil.
func NewDispatcher(
	queue *EmailQueue,
	sender EmailSender,
	calLister CalendarUpcomingLister,
	msgBus Publisher,
	stateManager StateManager,
) *Dispatcher {
	return &Dispatcher{
		queue:        queue,
		sender:       sender,
		calLister:    calLister,
		msgBus:       msgBus,
		stateManager: stateManager,
	}
}

// DispatchPending fetches all due emails from the queue and sends them.
// Designed to be registered with the shared Scheduler:
//
//	sched.Register("email_dispatch", 60*time.Second, disp.DispatchPending)
func (d *Dispatcher) DispatchPending(ctx context.Context) {
	if d.queue == nil || d.sender == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()

	pending, err := d.queue.GetPending(ctx)
	if err != nil {
		logger.ErrorCF("dispatcher", "Failed to get pending emails", map[string]any{"error": err.Error()})
		return
	}

	for _, email := range pending {
		to := []string{email.To}
		err := d.sender(ctx, to, email.Subject, email.Body, false, nil)
		if err != nil {
			logger.ErrorCF("dispatcher", "Failed to send email", map[string]any{
				"id":    email.ID,
				"to":    email.To,
				"error": err.Error(),
			})
			if markErr := d.queue.MarkError(ctx, email.ID, err.Error()); markErr != nil {
				logger.ErrorCF("dispatcher", "Failed to mark email as failed", map[string]any{"error": markErr.Error()})
			}
			continue
		}

		logger.InfoCF("dispatcher", "Email sent", map[string]any{"id": email.ID, "to": email.To})
		if markErr := d.queue.MarkSent(ctx, email.ID); markErr != nil {
			logger.ErrorCF("dispatcher", "Failed to mark email as sent", map[string]any{"error": markErr.Error()})
		}
	}
}

// CheckUpcomingEvents fetches upcoming calendar events and sends 15-minute
// reminders via the message bus. Designed to be registered with the shared Scheduler:
//
//	sched.Register("meeting_reminder", 5*time.Minute, disp.CheckUpcomingEvents)
func (d *Dispatcher) CheckUpcomingEvents(ctx context.Context) {
	if d.calLister == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	events, err := d.calLister(ctx, 20*time.Minute)
	if err != nil {
		logger.ErrorCF("dispatcher", "Failed to fetch upcoming events", map[string]any{"error": err.Error()})
		return
	}

	now := time.Now()
	for _, e := range events {
		startTime, err := time.Parse(time.RFC3339, e.StartRFC)
		if err != nil {
			continue
		}

		// Check if event starts in approximately 15 minutes (between 13 and 17 minutes from now).
		minsUntil := startTime.Sub(now).Minutes()
		if minsUntil < 13 || minsUntil > 17 {
			continue
		}

		// Skip if already notified.
		if d.stateManager != nil && d.stateManager.IsEventNotified(e.ID) {
			continue
		}

		msg := fmt.Sprintf("⏰ Reminder: %q starts in ~15 minutes (at %s)",
			e.Title, startTime.Format("3:04 PM"))

		d.msgBus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel: "all",
			ChatID:  "all",
			Content: msg,
		})

		if d.stateManager != nil {
			if err := d.stateManager.MarkEventNotified(e.ID); err != nil {
				logger.ErrorCF("dispatcher", "Failed to mark event notified", map[string]any{"error": err.Error()})
			}
		}

		logger.InfoCF("dispatcher", "Meeting reminder sent", map[string]any{"event": e.Title})
	}
}
