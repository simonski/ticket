package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultQueueStrategy           = "priority"
	DefaultForecastLookbackHours   = 24
	DefaultInterventionEscalationH = 24
	DefaultForecastMinAccuracyPct  = 60
)

type AutomationPolicy struct {
	QueueStrategy           string `json:"queue_strategy"`
	ForecastLookbackHours   int    `json:"forecast_lookback_hours"`
	InterventionEscalationH int    `json:"intervention_escalation_hours"`
	ForecastMinAccuracyPct  int    `json:"forecast_min_accuracy_pct"`
}

type TicketPolicyDiagnostics struct {
	TicketID      string   `json:"ticket_id"`
	QueueStrategy string   `json:"queue_strategy"`
	Eligible      bool     `json:"eligible"`
	Reasons       []string `json:"reasons,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

func normalizeQueueStrategy(raw string) (string, error) {
	strategy := strings.ToLower(strings.TrimSpace(raw))
	if strategy == "" {
		return DefaultQueueStrategy, nil
	}
	switch strategy {
	case "priority", "order", "aging":
		return strategy, nil
	default:
		return "", fmt.Errorf("invalid queue strategy %q", raw)
	}
}

func GetAutomationPolicy(ctx context.Context, db *sql.DB) (AutomationPolicy, error) {
	policy := AutomationPolicy{
		QueueStrategy:           DefaultQueueStrategy,
		ForecastLookbackHours:   DefaultForecastLookbackHours,
		InterventionEscalationH: DefaultInterventionEscalationH,
		ForecastMinAccuracyPct:  DefaultForecastMinAccuracyPct,
	}
	rowset, err := db.QueryContext(ctx, `
		SELECT key, value
		FROM app_settings
		WHERE key IN ('automation_queue_strategy', 'automation_forecast_lookback_hours', 'automation_intervention_escalation_hours', 'automation_forecast_min_accuracy_pct')
	`)
	if err != nil {
		return AutomationPolicy{}, err
	}
	defer rowset.Close()
	for rowset.Next() {
		var key, value string
		if scanErr := rowset.Scan(&key, &value); scanErr != nil {
			return AutomationPolicy{}, scanErr
		}
		switch key {
		case "automation_queue_strategy":
			strategy, normalizeErr := normalizeQueueStrategy(value)
			if normalizeErr == nil {
				policy.QueueStrategy = strategy
			}
		case "automation_forecast_lookback_hours":
			if parsed := parsePositiveInt(value); parsed > 0 {
				policy.ForecastLookbackHours = parsed
			}
		case "automation_intervention_escalation_hours":
			if parsed := parsePositiveInt(value); parsed > 0 {
				policy.InterventionEscalationH = parsed
			}
		case "automation_forecast_min_accuracy_pct":
			if parsed := parsePositiveInt(value); parsed > 0 && parsed <= 100 {
				policy.ForecastMinAccuracyPct = parsed
			}
		}
	}
	return policy, rowset.Err()
}

func SetAutomationPolicy(ctx context.Context, db *sql.DB, policy AutomationPolicy) (AutomationPolicy, error) {
	strategy, err := normalizeQueueStrategy(policy.QueueStrategy)
	if err != nil {
		return AutomationPolicy{}, err
	}
	if policy.ForecastLookbackHours <= 0 {
		policy.ForecastLookbackHours = DefaultForecastLookbackHours
	}
	if policy.InterventionEscalationH <= 0 {
		policy.InterventionEscalationH = DefaultInterventionEscalationH
	}
	if policy.ForecastMinAccuracyPct <= 0 || policy.ForecastMinAccuracyPct > 100 {
		policy.ForecastMinAccuracyPct = DefaultForecastMinAccuracyPct
	}
	updates := map[string]string{
		"automation_queue_strategy":                strategy,
		"automation_forecast_lookback_hours":       fmt.Sprintf("%d", policy.ForecastLookbackHours),
		"automation_intervention_escalation_hours": fmt.Sprintf("%d", policy.InterventionEscalationH),
		"automation_forecast_min_accuracy_pct":     fmt.Sprintf("%d", policy.ForecastMinAccuracyPct),
	}
	for key, value := range updates {
		if _, execErr := db.ExecContext(ctx, `
			INSERT INTO app_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, key, value); execErr != nil {
			return AutomationPolicy{}, execErr
		}
	}
	policy.QueueStrategy = strategy
	return policy, nil
}

func DiagnoseTicketPolicy(ctx context.Context, db *sql.DB, ticketID string) (TicketPolicyDiagnostics, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return TicketPolicyDiagnostics{}, err
	}
	policy, err := GetAutomationPolicy(ctx, db)
	if err != nil {
		return TicketPolicyDiagnostics{}, err
	}
	diag := TicketPolicyDiagnostics{
		TicketID:      ticket.ID,
		QueueStrategy: policy.QueueStrategy,
		Eligible:      true,
		Reasons:       make([]string, 0),
		Warnings:      make([]string, 0),
	}
	if ticket.Archived || ticket.Deleted {
		diag.Eligible = false
		diag.Reasons = append(diag.Reasons, "ticket is archived or deleted")
	}
	if ticket.Complete {
		diag.Eligible = false
		diag.Reasons = append(diag.Reasons, "ticket is complete")
	}
	if ticket.Draft {
		diag.Eligible = false
		diag.Reasons = append(diag.Reasons, "ticket is draft")
	}
	if strings.TrimSpace(ticket.Assignee) != "" {
		diag.Eligible = false
		diag.Reasons = append(diag.Reasons, "ticket already assigned")
	}
	if strings.EqualFold(strings.TrimSpace(ticket.State), StateFail) {
		diag.Eligible = false
		diag.Reasons = append(diag.Reasons, "ticket is in fail state and requires intervention")
	}
	deps, depErr := ListDependencies(ctx, db, ticket.ID)
	if depErr != nil {
		return TicketPolicyDiagnostics{}, depErr
	}
	for _, dep := range deps {
		blocker, blockerErr := GetTicket(ctx, db, dep.DependsOn)
		if blockerErr != nil {
			if errors.Is(blockerErr, ErrTicketNotFound) {
				diag.Warnings = append(diag.Warnings, "dependency "+dep.DependsOn+" cannot be resolved")
				continue
			}
			return TicketPolicyDiagnostics{}, blockerErr
		}
		if !blocker.Complete && !blocker.Archived {
			diag.Eligible = false
			diag.Reasons = append(diag.Reasons, "blocked by dependency "+dep.DependsOn)
			break
		}
	}
	return diag, nil
}
