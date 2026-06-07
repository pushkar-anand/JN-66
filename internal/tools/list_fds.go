package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// ListFDs lists fixed deposits for a user.
type ListFDs struct {
	userID string
	fds    fdLister
}

// NewListFDs creates the tool bound to the current user.
func NewListFDs(userID string, fds fdLister) *ListFDs {
	return &ListFDs{userID: userID, fds: fds}
}

// Definition returns the tool descriptor.
func (t *ListFDs) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "list_fds",
		Description: "List fixed deposits for a user. By default returns active FDs sorted by maturity date. Use status=all for full history including matured and renewed FDs.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id":              map[string]any{"type": "string", "description": "User ID (defaults to current user)"},
				"status":               map[string]any{"type": "string", "enum": []string{"active", "matured", "prematurely_closed", "renewed", "all"}, "description": "Filter by status (default: active)"},
				"maturing_within_days": map[string]any{"type": "integer", "description": "Only include FDs maturing within this many days from today"},
			},
		},
	}
}

type listFDsArgs struct {
	UserID             string `json:"user_id"`
	Status             string `json:"status"`
	MaturingWithinDays int    `json:"maturing_within_days"`
}

// Execute returns a formatted FD list.
func (t *ListFDs) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args listFDsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	userID := cmp.Or(args.UserID, t.userID)

	var statusFilter *sqlcgen.FdStatusEnum
	if args.Status != "" && args.Status != "all" {
		s := sqlcgen.FdStatusEnum(args.Status)
		if !s.Valid() {
			return "", fmt.Errorf("invalid status %q", args.Status)
		}
		statusFilter = &s
	} else if args.Status == "" {
		s := sqlcgen.FdStatusEnumActive
		statusFilter = &s
	}

	var maturingBefore *time.Time
	if args.MaturingWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, args.MaturingWithinDays)
		maturingBefore = &cutoff
	}

	fds, err := t.fds.ListByUser(ctx, store.ListFDsParams{
		UserID:         userID,
		Status:         statusFilter,
		MaturingBefore: maturingBefore,
	})
	if err != nil {
		return "", fmt.Errorf("list fds: %w", err)
	}
	slog.DebugContext(ctx, "tool:list_fds done", slog.Int("fds", len(fds)))

	if len(fds) == 0 {
		return "No fixed deposits found.", nil
	}

	var sb strings.Builder
	today := time.Now()

	var totalPrincipal, totalExpectedMaturity model.Money
	fmt.Fprintf(&sb, "Fixed Deposits (%d):\n\n", len(fds))

	for _, fd := range fds {
		daysLeft := int(fd.MaturityDate.Time.Sub(today).Hours() / 24)
		maturityStr := fd.MaturityDate.Time.Format("2 Jan 2006")
		if daysLeft > 0 {
			maturityStr = fmt.Sprintf("%s (%d days)", maturityStr, daysLeft)
		} else if daysLeft == 0 {
			maturityStr = maturityStr + " (today)"
		} else {
			maturityStr = fmt.Sprintf("%s (%d days ago)", maturityStr, -daysLeft)
		}

		fdNum := ""
		if fd.BankFdNumber != nil {
			fdNum = " [" + sanitizeField(*fd.BankFdNumber) + "]"
		}

		fmt.Fprintf(&sb, "• %s%s  %s @ %.2f%%  %d months\n",
			fd.ID,
			fdNum,
			model.Money(fd.PrincipalAmount).String(),
			float64(fd.InterestRateBps)/100,
			fd.TenureMonths,
		)
		fmt.Fprintf(&sb, "  %s → %s  Payout: %s  Renewal: %s  Status: %s\n",
			fd.StartDate.Time.Format("2 Jan 2006"),
			maturityStr,
			fd.InterestPayout,
			fd.AutoRenewalType,
			fd.Status,
		)
		if fd.ExpectedMaturityAmount > 0 {
			fmt.Fprintf(&sb, "  Expected maturity: %s\n", model.Money(fd.ExpectedMaturityAmount).String())
		}
		if fd.ActualPayoutAmount > 0 {
			fmt.Fprintf(&sb, "  Actual payout: %s\n", model.Money(fd.ActualPayoutAmount).String())
		}

		totalPrincipal += fd.PrincipalAmount
		totalExpectedMaturity += fd.ExpectedMaturityAmount
	}

	fmt.Fprintf(&sb, "\nTotal principal: %s", totalPrincipal.String())
	if totalExpectedMaturity > 0 {
		fmt.Fprintf(&sb, "  |  Total expected maturity: %s", totalExpectedMaturity.String())
	}

	return sb.String(), nil
}
