package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	bwglogger "github.com/pushkar-anand/build-with-go/logger"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// fdStoreAPI is the FD-store surface needed by the FD handler.
type fdStoreAPI interface {
	CreateWithAccount(ctx context.Context, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error)
}

// FDConfig holds dependencies for the FD API routes.
// Pass nil to api.New to disable these routes.
type FDConfig struct {
	Store fdStoreAPI
}

type createFDRequest struct {
	Institution            string  `json:"institution"              validate:"required"`
	AccountName            string  `json:"account_name"`
	BankFDNumber           string  `json:"bank_fd_number"`
	PrincipalAmount        float64 `json:"principal_amount"         validate:"required,gt=0"`
	InterestRate           float64 `json:"interest_rate"            validate:"required,gt=0"`
	TenureMonths           int     `json:"tenure_months"            validate:"required,gt=0"`
	StartDate              string  `json:"start_date"               validate:"required"`
	MaturityDate           string  `json:"maturity_date"            validate:"required"`
	ExpectedMaturityAmount float64 `json:"expected_maturity_amount"`
	InterestPayout         string  `json:"interest_payout"`
	AutoRenewalType        string  `json:"auto_renewal_type"`
	Notes                  string  `json:"notes"`
}

type createFDResponse struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
}

func (s *Server) handleCreateFD(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createFDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := s.v.ValidateStruct(r.Context(), &req)
	if err != nil {
		slog.ErrorContext(r.Context(), "fd create: validator error", bwglogger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !result.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(result.Failed)
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		http.Error(w, "invalid start_date: use YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	maturityDate, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		http.Error(w, "invalid maturity_date: use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	payout := sqlcgen.FdPayoutEnumCumulative
	if req.InterestPayout != "" {
		p := sqlcgen.FdPayoutEnum(req.InterestPayout)
		if !p.Valid() {
			http.Error(w, "invalid interest_payout", http.StatusBadRequest)
			return
		}
		payout = p
	}

	renewal := sqlcgen.FdRenewalTypeEnumNone
	if req.AutoRenewalType != "" {
		rn := sqlcgen.FdRenewalTypeEnum(req.AutoRenewalType)
		if !rn.Valid() {
			http.Error(w, "invalid auto_renewal_type", http.StatusBadRequest)
			return
		}
		renewal = rn
	}

	var bankFDNum *string
	if req.BankFDNumber != "" {
		bankFDNum = &req.BankFDNumber
	}
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}
	accountName := req.AccountName
	if accountName == "" {
		accountName = req.Institution + " FD"
	}

	userID := UserIDFromContext(r.Context())
	fd, err := s.fdCfg.Store.CreateWithAccount(r.Context(), store.CreateFDParams{
		UserID:                 userID,
		Institution:            req.Institution,
		AccountName:            accountName,
		BankFDNumber:           bankFDNum,
		PrincipalAmount:        model.Money(math.Round(req.PrincipalAmount * 100)),
		InterestRateBps:        int16(math.Round(req.InterestRate * 100)),
		TenureMonths:           int16(req.TenureMonths),
		StartDate:              startDate,
		MaturityDate:           maturityDate,
		ExpectedMaturityAmount: model.Money(math.Round(req.ExpectedMaturityAmount * 100)),
		InterestPayout:         payout,
		AutoRenewalType:        renewal,
		Notes:                  notes,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "fd create: store error",
			slog.String("user_id", userID),
			bwglogger.Error(err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "fd created",
		slog.String("user_id", userID),
		slog.String("fd_id", fd.ID.String()),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createFDResponse{
		ID:        fd.ID.String(),
		AccountID: fd.AccountID.String(),
	})
}
