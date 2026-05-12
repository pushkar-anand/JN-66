package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/zerodha"
)

// ErrZerodhaTokenExpired is returned when the stored Zerodha access token is
// missing or has passed its expiry. The caller should instruct the user to
// run: finagent zerodha auth
var ErrZerodhaTokenExpired = errors.New("zerodha token expired")

const holdingsCacheTTL = 4 * time.Hour

// ist is the Indian Standard Time fixed offset (+5:30).
var ist = time.FixedZone("IST", 5*60*60+30*60)

// ZerodhaStore wraps the sqlc querier for Zerodha-specific DB operations.
type ZerodhaStore struct {
	DB
	q sqlcgen.Querier
}

// NewZerodhaStore creates a ZerodhaStore backed by pool.
func NewZerodhaStore(pool *pgxpool.Pool) *ZerodhaStore {
	return &ZerodhaStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// GetToken retrieves the stored Zerodha access token for a user.
func (s *ZerodhaStore) GetToken(ctx context.Context, userID uuid.UUID) (*sqlcgen.ZerodhaToken, error) {
	tok, err := s.q.GetZerodhaToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get zerodha token: %w", err)
	}
	return &tok, nil
}

// UpsertToken stores or refreshes the Zerodha access token.
func (s *ZerodhaStore) UpsertToken(ctx context.Context, userID uuid.UUID, accessToken string, expiresAt time.Time) error {
	return s.q.UpsertZerodhaToken(ctx, sqlcgen.UpsertZerodhaTokenParams{
		UserID:      userID,
		AccessToken: accessToken,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
}

// FindOrCreateZerodhaAccount returns the Zerodha brokerage account ID for a user,
// creating one (with AddAccountMember) if it does not yet exist.
func (s *ZerodhaStore) FindOrCreateZerodhaAccount(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	accounts, err := s.q.ListAccountsByUser(ctx, userID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("list accounts: %w", err)
	}
	for _, a := range accounts {
		if strings.EqualFold(a.Institution, "Zerodha") {
			return a.ID, nil
		}
	}
	a, err := s.q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		Institution: "Zerodha",
		Name:        "Zerodha",
		AccountType: sqlcgen.AccountTypeEnumBrokerage,
		Currency:    "INR",
		IsActive:    true,
		Metadata:    []byte("{}"),
	})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("create zerodha account: %w", err)
	}
	if err := s.q.AddAccountMember(ctx, sqlcgen.AddAccountMemberParams{
		AccountID: a.ID,
		UserID:    userID,
		Role:      sqlcgen.MemberRoleEnumOwner,
	}); err != nil {
		return uuid.UUID{}, fmt.Errorf("add account member: %w", err)
	}
	return a.ID, nil
}

// ReplaceEquityHoldings atomically deletes and re-inserts all equity holdings
// for an account within a single transaction.
func (s *ZerodhaStore) ReplaceEquityHoldings(ctx context.Context, userID, accountID uuid.UUID, holdings []zerodha.Holding) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := sqlcgen.New(tx)
	if err := q.DeleteZerodhaEquityHoldings(ctx, accountID); err != nil {
		return fmt.Errorf("delete equity holdings: %w", err)
	}
	for _, h := range holdings {
		var isin *string
		if h.ISIN != "" {
			isin = &h.ISIN
		}
		if err := q.InsertZerodhaEquityHolding(ctx, sqlcgen.InsertZerodhaEquityHoldingParams{
			UserID:         userID,
			AccountID:      accountID,
			Tradingsymbol:  h.Tradingsymbol,
			Exchange:       h.Exchange,
			Isin:           isin,
			Quantity:       int32(h.Quantity),
			AvgPricePaise:  rupeesToPaise(h.AvgPrice),
			LastPricePaise: rupeesToPaise(h.LastPrice),
			PnlPaise:       rupeesToPaise(h.PnL),
			DayChangePaise: rupeesToPaise(h.DayChange),
		}); err != nil {
			return fmt.Errorf("insert equity holding %s: %w", h.Tradingsymbol, err)
		}
	}
	return tx.Commit(ctx)
}

// ReplaceMFHoldings atomically deletes and re-inserts all MF holdings for an account.
func (s *ZerodhaStore) ReplaceMFHoldings(ctx context.Context, userID, accountID uuid.UUID, holdings []zerodha.MFHolding) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := sqlcgen.New(tx)
	if err := q.DeleteZerodhaMFHoldings(ctx, accountID); err != nil {
		return fmt.Errorf("delete mf holdings: %w", err)
	}
	for _, h := range holdings {
		pnl := int64(math.Round((h.NAV - h.AverageNAV) * h.Units * 100))
		var units pgtype.Numeric
		if err := units.Scan(fmt.Sprintf("%.4f", h.Units)); err != nil {
			return fmt.Errorf("convert units for %s: %w", h.Fund, err)
		}
		if err := q.InsertZerodhaMFHolding(ctx, sqlcgen.InsertZerodhaMFHoldingParams{
			UserID:        userID,
			AccountID:     accountID,
			Folio:         h.Folio,
			Fund:          h.Fund,
			Tradingsymbol: h.Tradingsymbol,
			Units:         units,
			AvgNavPaise:   rupeesToPaise(h.AverageNAV),
			NavPaise:      rupeesToPaise(h.NAV),
			PnlPaise:      pnl,
		}); err != nil {
			return fmt.Errorf("insert mf holding %s: %w", h.Fund, err)
		}
	}
	return tx.Commit(ctx)
}

// equitySyncedAt returns the last equity sync timestamp for an account,
// or zero time if no holdings exist yet.
func (s *ZerodhaStore) equitySyncedAt(ctx context.Context, accountID uuid.UUID) time.Time {
	v, err := s.q.GetZerodhaEquitySyncedAt(ctx, accountID)
	if err != nil || v == nil {
		return time.Time{}
	}
	if ts, ok := v.(time.Time); ok {
		return ts
	}
	return time.Time{}
}

func (s *ZerodhaStore) getEquityHoldings(ctx context.Context, userID uuid.UUID) ([]sqlcgen.ZerodhaEquityHolding, error) {
	return s.q.ListZerodhaEquityHoldings(ctx, userID)
}

func (s *ZerodhaStore) getMFHoldings(ctx context.Context, userID uuid.UUID) ([]sqlcgen.ZerodhaMfHolding, error) {
	return s.q.ListZerodhaMFHoldings(ctx, userID)
}

func (s *ZerodhaStore) getEquitySummary(ctx context.Context, userID uuid.UUID) (sqlcgen.GetZerodhaEquitySummaryRow, error) {
	return s.q.GetZerodhaEquitySummary(ctx, userID)
}

func (s *ZerodhaStore) getMFSummary(ctx context.Context, userID uuid.UUID) (sqlcgen.GetZerodhaMFSummaryRow, error) {
	return s.q.GetZerodhaMFSummary(ctx, userID)
}

func (s *ZerodhaStore) getEquityHoldingsByType(ctx context.Context, userID uuid.UUID) ([]sqlcgen.GetZerodhaEquityHoldingsByTypeRow, error) {
	return s.q.GetZerodhaEquityHoldingsByType(ctx, userID)
}

func (s *ZerodhaStore) updateAccountBalance(ctx context.Context, accountID uuid.UUID, totalPaise int64) error {
	return s.q.UpdateAccountBalance(ctx, sqlcgen.UpdateAccountBalanceParams{
		ID:      accountID,
		Balance: model.Money(totalPaise),
		AsOf:    pgDate(time.Now()),
	})
}

func rupeesToPaise(r float64) int64 { return int64(math.Round(r * 100)) }

// ---------------------------------------------------------------------------
// ZerodhaService — composes ZerodhaStore + Zerodha HTTP client
// ---------------------------------------------------------------------------

// ZerodhaService implements lazy-sync: on each query it checks whether the
// cached holdings are stale (older than holdingsCacheTTL) and re-fetches from
// the Kite Connect API if needed.
type ZerodhaService struct {
	store  *ZerodhaStore
	client *zerodha.Client
}

// NewZerodhaService creates a ZerodhaService.
func NewZerodhaService(store *ZerodhaStore, client *zerodha.Client) *ZerodhaService {
	return &ZerodhaService{store: store, client: client}
}

// UpsertToken stores the exchanged access token. Expiry is set to midnight IST
// of the next calendar day, matching Zerodha's token lifetime.
func (s *ZerodhaService) UpsertToken(ctx context.Context, userID uuid.UUID, resp *zerodha.TokenResponse) error {
	now := time.Now().In(ist)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, ist)
	return s.store.UpsertToken(ctx, userID, resp.AccessToken, midnight)
}

// GetToken retrieves the raw token row (used by auth handlers).
func (s *ZerodhaService) GetToken(ctx context.Context, userID uuid.UUID) (*sqlcgen.ZerodhaToken, error) {
	return s.store.GetToken(ctx, userID)
}

// ForceSync fetches fresh holdings from Zerodha and replaces the cached data.
// Returns ErrZerodhaTokenExpired if the stored token is missing or expired.
func (s *ZerodhaService) ForceSync(ctx context.Context, userID uuid.UUID) (equityCount, mfCount int, err error) {
	accessToken, err := s.loadToken(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	accountID, err := s.store.FindOrCreateZerodhaAccount(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	return s.doSync(ctx, userID, accountID, accessToken)
}

// GetEquityHoldings returns equity + SGB holdings, triggering a sync if stale.
func (s *ZerodhaService) GetEquityHoldings(ctx context.Context, userID string) ([]sqlcgen.ZerodhaEquityHolding, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureFresh(ctx, uid); err != nil {
		return nil, err
	}
	return s.store.getEquityHoldings(ctx, uid)
}

// GetMFHoldings returns mutual fund holdings, triggering a sync if stale.
func (s *ZerodhaService) GetMFHoldings(ctx context.Context, userID string) ([]sqlcgen.ZerodhaMfHolding, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureFresh(ctx, uid); err != nil {
		return nil, err
	}
	return s.store.getMFHoldings(ctx, uid)
}

// GetEquitySummary returns aggregate equity stats, triggering a sync if stale.
func (s *ZerodhaService) GetEquitySummary(ctx context.Context, userID string) (sqlcgen.GetZerodhaEquitySummaryRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return sqlcgen.GetZerodhaEquitySummaryRow{}, err
	}
	if err := s.ensureFresh(ctx, uid); err != nil {
		return sqlcgen.GetZerodhaEquitySummaryRow{}, err
	}
	return s.store.getEquitySummary(ctx, uid)
}

// GetMFSummary returns aggregate MF stats, triggering a sync if stale.
func (s *ZerodhaService) GetMFSummary(ctx context.Context, userID string) (sqlcgen.GetZerodhaMFSummaryRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return sqlcgen.GetZerodhaMFSummaryRow{}, err
	}
	if err := s.ensureFresh(ctx, uid); err != nil {
		return sqlcgen.GetZerodhaMFSummaryRow{}, err
	}
	return s.store.getMFSummary(ctx, uid)
}

// GetEquityHoldingsByType returns equity holdings broken down by type (equity vs SGB).
func (s *ZerodhaService) GetEquityHoldingsByType(ctx context.Context, userID string) ([]sqlcgen.GetZerodhaEquityHoldingsByTypeRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureFresh(ctx, uid); err != nil {
		return nil, err
	}
	return s.store.getEquityHoldingsByType(ctx, uid)
}

// ensureFresh checks whether holdings are stale and syncs from the API if needed.
func (s *ZerodhaService) ensureFresh(ctx context.Context, userID uuid.UUID) error {
	accountID, err := s.store.FindOrCreateZerodhaAccount(ctx, userID)
	if err != nil {
		return err
	}
	syncedAt := s.store.equitySyncedAt(ctx, accountID)
	if time.Since(syncedAt) < holdingsCacheTTL {
		return nil
	}
	accessToken, err := s.loadToken(ctx, userID)
	if err != nil {
		return err
	}
	_, _, err = s.doSync(ctx, userID, accountID, accessToken)
	return err
}

// doSync fetches holdings from the API and replaces the DB cache.
func (s *ZerodhaService) doSync(ctx context.Context, userID, accountID uuid.UUID, accessToken string) (equityCount, mfCount int, err error) {
	holdings, err := s.client.GetHoldings(ctx, accessToken)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch equity holdings: %w", err)
	}
	if err := s.store.ReplaceEquityHoldings(ctx, userID, accountID, holdings); err != nil {
		return 0, 0, err
	}

	mfHoldings, err := s.client.GetMFHoldings(ctx, accessToken)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch mf holdings: %w", err)
	}
	if err := s.store.ReplaceMFHoldings(ctx, userID, accountID, mfHoldings); err != nil {
		return 0, 0, err
	}

	// Update the Zerodha account balance to total portfolio value.
	eqSummary, _ := s.store.getEquitySummary(ctx, userID)
	mfSummary, _ := s.store.getMFSummary(ctx, userID)
	totalPaise := eqSummary.CurrentValuePaise + mfSummary.CurrentValuePaise
	_ = s.store.updateAccountBalance(ctx, accountID, totalPaise)

	return len(holdings), len(mfHoldings), nil
}

// loadToken retrieves the stored access token, returning ErrZerodhaTokenExpired
// if it is missing or past its expiry time.
func (s *ZerodhaService) loadToken(ctx context.Context, userID uuid.UUID) (string, error) {
	tok, err := s.store.GetToken(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrZerodhaTokenExpired
		}
		return "", fmt.Errorf("load token: %w", err)
	}
	if !tok.ExpiresAt.Valid || time.Now().After(tok.ExpiresAt.Time) {
		return "", ErrZerodhaTokenExpired
	}
	return tok.AccessToken, nil
}
