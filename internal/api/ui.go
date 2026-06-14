package api

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pushkar-anand/build-with-go/logger"

	"github.com/pushkaranand/finagent/internal/apikey"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

//go:embed ui/templates
var uiTemplates embed.FS

const uiSessionCookie = "jn66_session"

// uiUserStore is the subset of user store needed for UI session auth.
type uiUserStore interface {
	GetByAPIKeyPrefix(ctx context.Context, prefix string) (*sqlcgen.User, error)
}

// uiAccountStore is the subset of account store needed by the UI.
type uiAccountStore interface {
	ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error)
}

// uiFDStore is the subset of FD store needed by the UI.
type uiFDStore interface {
	ListByUser(ctx context.Context, p store.ListFDsParams) ([]sqlcgen.FixedDeposit, error)
}

// uiZerodhaStore is the subset of Zerodha store needed by the UI.
type uiZerodhaStore interface {
	GetEquityHoldings(ctx context.Context, userID string) ([]sqlcgen.ZerodhaEquityHolding, error)
	GetMFHoldings(ctx context.Context, userID string) ([]sqlcgen.ZerodhaMfHolding, error)
}

// UIConfig holds dependencies for the HTML UI routes.
// Pass nil to api.New to disable the UI routes.
type UIConfig struct {
	UserStore    uiUserStore
	AccountStore uiAccountStore
	FDStore      uiFDStore
	ZerodhaStore uiZerodhaStore // nil → Zerodha sections hidden
}

// Page-specific template sets — each page clones base+partials so that
// their {{define "content"}} blocks do not clobber each other.
var (
	tmplLogin *template.Template // standalone login page
	tmplTxn   *template.Template // transactions page + partials (reused for HTMX responses)
	tmplAcct  *template.Template // accounts page
	tmplInv   *template.Template // investments page
)

func init() {
	funcMap := template.FuncMap{
		// paiseRupees formats paise int64 as a rupee string using model.Money.String().
		"paiseRupees": func(p int64) string { return model.Money(p).String() },
		// dict creates a map[string]any from alternating key, value pairs.
		"dict": func(kv ...any) map[string]any {
			m := make(map[string]any, len(kv)/2)
			for i := 0; i+1 < len(kv); i += 2 {
				if k, ok := kv[i].(string); ok {
					m[k] = kv[i+1]
				}
			}
			return m
		},
		"add": func(a, b int) int { return a + b },
		"divCeil": func(a int64, b int64) int64 {
			if b == 0 {
				return 0
			}
			return (a + b - 1) / b
		},
		"int64": func(n int) int64 { return int64(n) },
		"sub":   func(a, b int) int { return a - b },
		// deref dereferences a *string; returns "" if nil.
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
		// bps converts basis points (int16) to a float percentage.
		"bps": func(b int16) float64 { return float64(b) / 100 },
		// pgDate formats a pgtype.Date as "2006-01-02".
		"pgDate": func(d pgtype.Date) string {
			if !d.Valid {
				return "—"
			}
			return d.Time.Format("2006-01-02")
		},
	}

	// Login is standalone — no base template.
	tmplLogin = template.Must(
		template.New("login.html").Funcs(funcMap).ParseFS(uiTemplates, "ui/templates/login.html"),
	)

	// Base + partials — foundation cloned for each page so their "content"
	// definitions stay isolated from each other.
	base := template.Must(
		template.New("").Funcs(funcMap).ParseFS(uiTemplates,
			"ui/templates/base.html",
			"ui/templates/partials/*.html",
		),
	)
	clonePage := func(filename string) *template.Template {
		c := template.Must(base.Clone())
		return template.Must(c.ParseFS(uiTemplates, "ui/templates/"+filename))
	}
	tmplTxn  = clonePage("transactions.html")
	tmplAcct = clonePage("accounts.html")
	tmplInv  = clonePage("investments.html")
}

// formatRupees is used by buildUITxnRow to pre-format amounts for the row struct.
func formatRupees(p int64) string { return model.Money(p).String() }

// uiSessionMiddleware validates the jn66_session cookie and injects user ID.
func (s *Server) uiSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(uiSessionCookie)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
			return
		}
		token := cookie.Value
		prefix := apikey.Prefix(token)
		user, err := s.uiCfg.UserStore.GetByAPIKeyPrefix(r.Context(), prefix)
		if err != nil || !apikey.Verify(token, user.ApiKeyHash) {
			http.SetCookie(w, &http.Cookie{Name: uiSessionCookie, MaxAge: -1, Path: "/"})
			http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), user.ID.String())))
	})
}

// handleUILogin handles GET /ui/login — renders login form.
func (s *Server) handleUILogin(w http.ResponseWriter, r *http.Request) {
	_ = tmplLogin.ExecuteTemplate(w, "login.html", nil)
}

// handleUILoginPost handles POST /ui/login — validates API key, sets session cookie.
func (s *Server) handleUILoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	token := r.FormValue("api_key")
	if token == "" {
		_ = tmplLogin.ExecuteTemplate(w, "login.html", map[string]string{"Error": "API key required"})
		return
	}
	prefix := apikey.Prefix(token)
	user, err := s.uiCfg.UserStore.GetByAPIKeyPrefix(r.Context(), prefix)
	if err != nil || !apikey.Verify(token, user.ApiKeyHash) {
		_ = tmplLogin.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Invalid API key"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30, // 30 days
	})
	http.Redirect(w, r, "/ui/transactions", http.StatusSeeOther)
}

// handleUILogout handles GET /ui/logout — clears session cookie.
func (s *Server) handleUILogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: uiSessionCookie, MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
}

// uiTransactionsData is the template data for the transactions page.
type uiTransactionsData struct {
	Transactions  []uiTxnRow
	Categories    []sqlcgen.Category
	Accounts      []sqlcgen.Account
	Total         int64
	Page          int
	Limit         int
	FilterAccount string
	FilterCat     string
	FilterDir     string
	FilterFrom    string
	FilterTo      string
}

// uiTxnRow is a single displayable transaction row.
type uiTxnRow struct {
	ID                    string
	TxnDate               string
	Description           string
	DescriptionNormalized string
	Amount                string
	Direction             string
	CounterpartyName      string
	CounterpartyID        string
	CategoryID            string
	CategorySlug          string
	CategoryName          string
	Notes                 string
	TaggingStatus         string
	Labels                []sqlcgen.Label
}

func buildUITxnRow(t sqlcgen.VTransaction, catMap map[string]sqlcgen.Category, labels []sqlcgen.Label) uiTxnRow {
	row := uiTxnRow{
		ID:        t.ID.String(),
		Direction: string(t.Direction),
		Amount:    formatRupees(t.Amount),
		Labels:    labels,
	}
	if t.TxnDate.Valid {
		row.TxnDate = t.TxnDate.Time.Format("2006-01-02")
	}
	row.Description = t.Description
	if t.DescriptionNormalized != nil {
		row.DescriptionNormalized = *t.DescriptionNormalized
	}
	if t.CounterpartyName != nil {
		row.CounterpartyName = *t.CounterpartyName
	}
	if t.CounterpartyIdentifier != nil {
		row.CounterpartyID = *t.CounterpartyIdentifier
	}
	if t.Notes != nil {
		row.Notes = *t.Notes
	}
	if t.TaggingStatus != nil {
		row.TaggingStatus = string(*t.TaggingStatus)
	}
	if t.CategoryID.Valid {
		cid := uuidStr(t.CategoryID)
		row.CategoryID = cid
		if cat, ok := catMap[cid]; ok {
			row.CategorySlug = cat.Slug
			row.CategoryName = cat.Name
		}
	}
	return row
}

// handleUITransactions handles GET /ui/transactions — full transactions page.
func (s *Server) handleUITransactions(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	q := r.URL.Query()

	page := 1
	limit := 50
	if v := q.Get("page"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			page = n
		}
	}

	params := store.ListTransactionsParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32((page - 1) * limit),
	}
	if v := q.Get("account_id"); v != "" {
		params.AccountID = &v
	}
	if v := q.Get("category_id"); v != "" {
		params.CategoryID = &v
	}
	if v := q.Get("direction"); v != "" {
		d := sqlcgen.TxnDirectionEnum(v)
		params.Direction = &d
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			params.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			params.To = &t
		}
	}

	rows, err := s.txnCfg.TxnStore.List(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "ui: list transactions", logger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	total, _ := s.txnCfg.TxnStore.Count(r.Context(), params)
	cats, _ := s.txnCfg.CatStore.List(r.Context())
	accounts, _ := s.uiCfg.AccountStore.ListByUser(r.Context(), userID)

	catMap := make(map[string]sqlcgen.Category, len(cats))
	for _, c := range cats {
		catMap[c.ID.String()] = c
	}

	txnRows := make([]uiTxnRow, len(rows))
	for i, row := range rows {
		labels, _ := s.txnCfg.LabelStore.ListForTransaction(r.Context(), row.ID)
		txnRows[i] = buildUITxnRow(row, catMap, labels)
	}

	data := uiTransactionsData{
		Transactions:  txnRows,
		Categories:    cats,
		Accounts:      accounts,
		Total:         total,
		Page:          page,
		Limit:         limit,
		FilterAccount: q.Get("account_id"),
		FilterCat:     q.Get("category_id"),
		FilterDir:     q.Get("direction"),
		FilterFrom:    q.Get("from"),
		FilterTo:      q.Get("to"),
	}

	_ = tmplTxn.ExecuteTemplate(w, "transactions.html", data)
}

// handleUIEnrich handles POST /ui/transactions/{id}/enrich — HTMX form for inline edit.
func (s *Server) handleUIEnrich(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	txnID := mux.Vars(r)["id"]

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	txn, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	catSlug := r.FormValue("category_slug")
	notes := r.FormValue("notes")
	ep := store.EnrichmentParams{TransactionID: txnID, Notes: nilStr(notes)}
	manual := sqlcgen.TaggingStatusEnumManual
	ep.TaggingStatus = &manual

	var newSlug string
	if catSlug != "" {
		cat, err := s.txnCfg.CatStore.GetBySlug(r.Context(), catSlug)
		if err == nil {
			id := cat.ID.String()
			ep.CategoryID = &id
			newSlug = cat.Slug
		}
	}

	if err := s.txnCfg.TxnStore.UpdateEnrichment(r.Context(), ep); err != nil {
		slog.ErrorContext(r.Context(), "ui: update enrichment", logger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-save tagging hint memory when category changed.
	if newSlug != "" && txn.CounterpartyIdentifier != nil && s.txnCfg.MemStore != nil {
		oldSlug := ""
		if txn.CategoryID.Valid {
			cats, _ := s.txnCfg.CatStore.List(r.Context())
			for _, c := range cats {
				if c.ID.String() == uuidStr(txn.CategoryID) {
					oldSlug = c.Slug
					break
				}
			}
		}
		if oldSlug != newSlug {
			hint := "Counterparty '" + *txn.CounterpartyIdentifier +
				"' (bank description: '" + txn.Description +
				"') → category '" + newSlug +
				"'. User manually corrected from '" + oldSlug + "'."
			uid := userID
			if _, err := s.txnCfg.MemStore.Save(r.Context(), &uid, hint,
				sqlcgen.MemoryTypeEnumTaggingHint, []string{*txn.CounterpartyIdentifier, newSlug},
			); err != nil {
				slog.WarnContext(r.Context(), "ui: save tagging hint", logger.Error(err))
			}
		}
	}

	// Re-fetch and render the updated row.
	updated, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	cats, _ := s.txnCfg.CatStore.List(r.Context())
	catMap := make(map[string]sqlcgen.Category, len(cats))
	for _, c := range cats {
		catMap[c.ID.String()] = c
	}
	labels, _ := s.txnCfg.LabelStore.ListForTransaction(r.Context(), updated.ID)
	row := buildUITxnRow(*updated, catMap, labels)

	w.Header().Set("Content-Type", "text/html")
	_ = tmplTxn.ExecuteTemplate(w, "txn_row.html", struct {
		Row        uiTxnRow
		Categories []sqlcgen.Category
	}{Row: row, Categories: cats})
}

// handleUIAddLabel handles POST /ui/transactions/{id}/label — adds a label via HTMX.
func (s *Server) handleUIAddLabel(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	txnID := mux.Vars(r)["id"]

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	if _, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	name := r.FormValue("label")
	if name == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}

	labelID, err := s.txnCfg.LabelStore.FindOrCreate(r.Context(), userID, name)
	if err != nil {
		slog.ErrorContext(r.Context(), "ui: find or create label", logger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	_ = s.txnCfg.LabelStore.AddToTransaction(r.Context(), txnID, labelID)

	s.renderLabelChips(w, r, txnID)
}

// handleUIRemoveLabel handles DELETE /ui/transactions/{id}/label/{labelId}.
func (s *Server) handleUIRemoveLabel(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	vars := mux.Vars(r)
	txnID := vars["id"]
	labelID := vars["labelId"]

	if _, err := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, userID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = s.txnCfg.LabelStore.RemoveFromTransaction(r.Context(), txnID, labelID)
	s.renderLabelChips(w, r, txnID)
}

func (s *Server) renderLabelChips(w http.ResponseWriter, r *http.Request, txnID string) {
	txn, _ := s.txnCfg.TxnStore.GetByID(r.Context(), txnID, UserIDFromContext(r.Context()))
	var labels []sqlcgen.Label
	if txn != nil {
		labels, _ = s.txnCfg.LabelStore.ListForTransaction(r.Context(), txn.ID)
	}
	w.Header().Set("Content-Type", "text/html")
	_ = tmplTxn.ExecuteTemplate(w, "label_chips.html", struct {
		TxnID  string
		Labels []sqlcgen.Label
	}{TxnID: txnID, Labels: labels})
}

// handleUIAccounts handles GET /ui/accounts.
func (s *Server) handleUIAccounts(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	accounts, err := s.uiCfg.AccountStore.ListByUser(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "ui: list accounts", logger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	_ = tmplAcct.ExecuteTemplate(w, "accounts.html", map[string]any{"Accounts": accounts})
}

// handleUIInvestments handles GET /ui/investments.
func (s *Server) handleUIInvestments(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	fds, err := s.uiCfg.FDStore.ListByUser(r.Context(), store.ListFDsParams{UserID: userID})
	if err != nil {
		slog.ErrorContext(r.Context(), "ui: list fds", logger.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{"FDs": fds}
	if s.uiCfg.ZerodhaStore != nil {
		equity, _ := s.uiCfg.ZerodhaStore.GetEquityHoldings(r.Context(), userID)
		mf, _ := s.uiCfg.ZerodhaStore.GetMFHoldings(r.Context(), userID)
		data["Equity"] = equity
		data["MF"] = mf
	}
	_ = tmplInv.ExecuteTemplate(w, "investments.html", data)
}

// parseInt parses a base-10 int from a string; returns 0 and error on failure.
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

type parseError struct{ s string }

func (e *parseError) Error() string { return "not an integer: " + e.s }

// nilStr returns a *string pointing to s, or nil if s is empty.
func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// uuidStr converts a pgtype.UUID to its string form.
func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}
