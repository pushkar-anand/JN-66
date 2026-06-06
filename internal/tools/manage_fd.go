package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// ManageFD creates and updates fixed deposits.
type ManageFD struct {
	userID string
	fds    fdManager
}

// NewManageFD creates the tool bound to the current user.
func NewManageFD(userID string, fds fdManager) *ManageFD {
	return &ManageFD{userID: userID, fds: fds}
}

// Definition returns the tool descriptor.
func (t *ManageFD) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name: "manage_fd",
		Description: "Create a fixed deposit or update its lifecycle status (matured, prematurely closed, or renewed). " +
			"Use action=create to record a new FD. Use mark_matured or mark_prematurely_closed when an FD ends. " +
			"Use mark_renewed when the bank auto-renews the FD at a new rate.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"create", "mark_matured", "mark_prematurely_closed", "mark_renewed"},
					"description": "Operation to perform.",
				},
				"user_id": map[string]any{"type": "string", "description": "User ID (defaults to current user)"},
				"fd_id":   map[string]any{"type": "string", "description": "ID of an existing FD (required for mark_* actions)"},
				// create fields
				"institution":              map[string]any{"type": "string", "description": "Bank name, e.g. hdfc, sbi"},
				"account_name":             map[string]any{"type": "string", "description": "Human-readable FD account name, e.g. 'HDFC FD 12M'"},
				"bank_fd_number":           map[string]any{"type": "string", "description": "Bank's FD reference number (optional)"},
				"principal_amount":         map[string]any{"type": "number", "description": "Principal amount in rupees"},
				"interest_rate":            map[string]any{"type": "number", "description": "Annual interest rate as a percentage, e.g. 7.5"},
				"tenure_months":            map[string]any{"type": "integer", "description": "Tenure in months"},
				"start_date":               map[string]any{"type": "string", "description": "Start date in YYYY-MM-DD format"},
				"maturity_date":            map[string]any{"type": "string", "description": "Maturity date in YYYY-MM-DD format"},
				"expected_maturity_amount": map[string]any{"type": "number", "description": "Expected maturity amount in rupees (optional, as shown by bank)"},
				"interest_payout":          map[string]any{"type": "string", "enum": []string{"cumulative", "monthly", "quarterly", "annual"}, "description": "How interest is paid out (default: cumulative)"},
				"auto_renewal_type": map[string]any{
					"type":        "string",
					"enum":        []string{"none", "principal_only", "principal_and_interest"},
					"description": "Renewal behaviour on maturity: none=payout only, principal_only=reinvest principal, principal_and_interest=reinvest full maturity amount",
				},
				"notes": map[string]any{"type": "string", "description": "Optional notes"},
				// mark_* fields
				"actual_payout_amount": map[string]any{"type": "number", "description": "Actual payout in rupees (required for mark_matured, mark_prematurely_closed, mark_renewed)"},
				// mark_renewed fields
				"new_interest_rate":  map[string]any{"type": "number", "description": "New annual interest rate for renewed FD (%)"},
				"new_tenure_months":  map[string]any{"type": "integer", "description": "New tenure in months for renewed FD"},
				"new_bank_fd_number": map[string]any{"type": "string", "description": "New bank FD reference number after renewal (optional)"},
			},
			"required": []string{"action"},
		},
	}
}

type manageFDArgs struct {
	Action      string `json:"action"`
	UserID      string `json:"user_id"`
	FDID        string `json:"fd_id"`
	Institution string `json:"institution"`
	AccountName string `json:"account_name"`
	BankFDNum   string `json:"bank_fd_number"`

	PrincipalAmount        float64 `json:"principal_amount"`
	InterestRate           float64 `json:"interest_rate"`
	TenureMonths           int     `json:"tenure_months"`
	StartDate              string  `json:"start_date"`
	MaturityDate           string  `json:"maturity_date"`
	ExpectedMaturityAmount float64 `json:"expected_maturity_amount"`
	InterestPayout         string  `json:"interest_payout"`
	AutoRenewalType        string  `json:"auto_renewal_type"`
	Notes                  string  `json:"notes"`

	ActualPayoutAmount float64 `json:"actual_payout_amount"`
	NewInterestRate    float64 `json:"new_interest_rate"`
	NewTenureMonths    int     `json:"new_tenure_months"`
	NewBankFDNum       string  `json:"new_bank_fd_number"`
}

// Execute runs the requested manage_fd action.
func (t *ManageFD) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args manageFDArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	userID := cmp.Or(args.UserID, t.userID)

	switch args.Action {
	case "create":
		return t.create(ctx, userID, args)
	case "mark_matured":
		return t.updateStatus(ctx, userID, args, sqlcgen.FdStatusEnumMatured)
	case "mark_prematurely_closed":
		return t.updateStatus(ctx, userID, args, sqlcgen.FdStatusEnumPrematurelyClosed)
	case "mark_renewed":
		return t.renew(ctx, userID, args)
	default:
		return "", fmt.Errorf("unknown action %q", args.Action)
	}
}

func (t *ManageFD) create(ctx context.Context, userID string, args manageFDArgs) (string, error) {
	if args.Institution == "" {
		return "", fmt.Errorf("institution is required")
	}
	if args.PrincipalAmount <= 0 {
		return "", fmt.Errorf("principal_amount is required")
	}
	if args.InterestRate <= 0 {
		return "", fmt.Errorf("interest_rate is required")
	}
	if args.InterestRate > 30 {
		return "", fmt.Errorf("interest_rate %g%% is unreasonably high for an FD (max 30%%)", args.InterestRate)
	}
	if args.TenureMonths <= 0 {
		return "", fmt.Errorf("tenure_months is required")
	}

	startDate, err := time.Parse("2006-01-02", args.StartDate)
	if err != nil {
		return "", fmt.Errorf("invalid start_date %q: %w", args.StartDate, err)
	}
	maturityDate, err := time.Parse("2006-01-02", args.MaturityDate)
	if err != nil {
		return "", fmt.Errorf("invalid maturity_date %q: %w", args.MaturityDate, err)
	}

	payout := sqlcgen.FdPayoutEnumCumulative
	if args.InterestPayout != "" {
		if !sqlcgen.FdPayoutEnum(args.InterestPayout).Valid() {
			return "", fmt.Errorf("invalid interest_payout %q", args.InterestPayout)
		}
		payout = sqlcgen.FdPayoutEnum(args.InterestPayout)
	}

	renewal := sqlcgen.FdRenewalTypeEnumNone
	if args.AutoRenewalType != "" {
		if !sqlcgen.FdRenewalTypeEnum(args.AutoRenewalType).Valid() {
			return "", fmt.Errorf("invalid auto_renewal_type %q", args.AutoRenewalType)
		}
		renewal = sqlcgen.FdRenewalTypeEnum(args.AutoRenewalType)
	}

	accountName := cmp.Or(args.AccountName, fmt.Sprintf("%s FD %dM", args.Institution, args.TenureMonths))
	var bankFDNum *string
	if args.BankFDNum != "" {
		bankFDNum = &args.BankFDNum
	}
	var notes *string
	if args.Notes != "" {
		notes = &args.Notes
	}

	fd, err := t.fds.CreateWithAccount(ctx, store.CreateFDParams{
		UserID:                 userID,
		Institution:            args.Institution,
		AccountName:            accountName,
		BankFDNumber:           bankFDNum,
		PrincipalAmount:        model.FromRupees(args.PrincipalAmount),
		InterestRateBps:        int16(math.Round(args.InterestRate * 100)),
		TenureMonths:           int16(args.TenureMonths),
		StartDate:              startDate,
		MaturityDate:           maturityDate,
		ExpectedMaturityAmount: model.FromRupees(args.ExpectedMaturityAmount),
		InterestPayout:         payout,
		AutoRenewalType:        renewal,
		Notes:                  notes,
	})
	if err != nil {
		return "", fmt.Errorf("create fd: %w", err)
	}

	return fmt.Sprintf("Fixed deposit created. ID: %s\nAccount: %s\nPrincipal: %s  Rate: %.2f%%  Tenure: %d months\nMatures: %s  Renewal: %s",
		fd.ID,
		accountName,
		model.Money(fd.PrincipalAmount).String(),
		float64(fd.InterestRateBps)/100,
		fd.TenureMonths,
		fd.MaturityDate.Time.Format("2 Jan 2006"),
		fd.AutoRenewalType,
	), nil
}

func (t *ManageFD) updateStatus(ctx context.Context, userID string, args manageFDArgs, status sqlcgen.FdStatusEnum) (string, error) {
	if args.FDID == "" {
		return "", fmt.Errorf("fd_id is required")
	}
	fd, err := t.fds.UpdateStatus(ctx, store.UpdateStatusParams{
		UserID:             userID,
		FDID:               args.FDID,
		Status:             status,
		ActualPayoutAmount: model.FromRupees(args.ActualPayoutAmount),
	})
	if err != nil {
		return "", fmt.Errorf("update fd: %w", err)
	}

	return fmt.Sprintf("Fixed deposit %s marked as %s. Payout: %s",
		fd.ID,
		fd.Status,
		model.Money(fd.ActualPayoutAmount).String(),
	), nil
}

func (t *ManageFD) renew(ctx context.Context, userID string, args manageFDArgs) (string, error) {
	if args.FDID == "" {
		return "", fmt.Errorf("fd_id is required")
	}
	if args.NewInterestRate <= 0 {
		return "", fmt.Errorf("new_interest_rate is required")
	}
	if args.NewInterestRate > 30 {
		return "", fmt.Errorf("new_interest_rate %g%% is unreasonably high for an FD (max 30%%)", args.NewInterestRate)
	}
	if args.NewTenureMonths <= 0 {
		return "", fmt.Errorf("new_tenure_months is required")
	}
	if args.ActualPayoutAmount <= 0 {
		return "", fmt.Errorf("actual_payout_amount is required")
	}
	if args.Institution == "" {
		return "", fmt.Errorf("institution is required for renewal")
	}

	old, err := t.fds.Get(ctx, args.FDID, userID)
	if err != nil {
		return "", fmt.Errorf("get fd: %w", err)
	}

	// new start = old maturity date; new maturity = start + new_tenure_months
	newStart := old.MaturityDate.Time
	newMaturity := newStart.AddDate(0, args.NewTenureMonths, 0)

	// new principal depends on renewal type
	newPrincipal := model.FromRupees(args.ActualPayoutAmount)
	if old.AutoRenewalType == sqlcgen.FdRenewalTypeEnumPrincipalOnly {
		newPrincipal = old.PrincipalAmount
	}
	var newBankFDNum *string
	if args.NewBankFDNum != "" {
		newBankFDNum = &args.NewBankFDNum
	}
	newAccountName := cmp.Or(args.AccountName, fmt.Sprintf("%s FD %dM (renewal)", args.Institution, args.NewTenureMonths))

	fd, err := t.fds.RenewFD(ctx, store.RenewFDParams{
		UserID:             userID,
		OldFDID:            args.FDID,
		ActualPayoutAmount: model.FromRupees(args.ActualPayoutAmount),
		Institution:        args.Institution,
		NewAccountName:     newAccountName,
		NewBankFDNumber:    newBankFDNum,
		NewPrincipalAmount: newPrincipal,
		NewInterestRateBps: int16(math.Round(args.NewInterestRate * 100)),
		NewTenureMonths:    int16(args.NewTenureMonths),
		NewStartDate:       newStart,
		NewMaturityDate:    newMaturity,
		NewInterestPayout:  old.InterestPayout,
		NewAutoRenewalType: old.AutoRenewalType,
	})
	if err != nil {
		return "", fmt.Errorf("renew fd: %w", err)
	}

	return fmt.Sprintf("FD renewed. Old FD %s marked as renewed.\nNew FD ID: %s\nPrincipal: %s  Rate: %.2f%%  Tenure: %d months\nMatures: %s",
		args.FDID,
		fd.ID,
		model.Money(fd.PrincipalAmount).String(),
		float64(fd.InterestRateBps)/100,
		fd.TenureMonths,
		fd.MaturityDate.Time.Format("2 Jan 2006"),
	), nil
}
