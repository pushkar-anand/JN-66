// Package zerodha provides a client for the Kite Connect API.
package zerodha

// Holding is one equity/ETF/SGB position from GET /portfolio/holdings.
// SGBs have no distinct instrument_type field; detect via strings.HasPrefix(Tradingsymbol, "SGB").
type Holding struct {
	Tradingsymbol string  `json:"tradingsymbol"`
	Exchange      string  `json:"exchange"`
	ISIN          string  `json:"isin"`
	Quantity      int     `json:"quantity"`
	AvgPrice      float64 `json:"average_price"`
	LastPrice     float64 `json:"last_price"`
	PnL           float64 `json:"pnl"`
	DayChange     float64 `json:"day_change"`
}

// MFHolding is one mutual fund position from GET /mf/holdings.
// Tradingsymbol is the ISIN (e.g. INF0R8F01026).
// NAV is the current NAV per unit in rupees — NOT the total value.
// PnL is always 0 from the API; compute as (NAV - AverageNAV) * Units.
type MFHolding struct {
	Tradingsymbol string  `json:"tradingsymbol"`
	Fund          string  `json:"fund"`
	Folio         string  `json:"folio"`
	Units         float64 `json:"quantity"`
	AverageNAV    float64 `json:"average_price"`
	NAV           float64 `json:"last_price"`
}

// TokenResponse is the payload from POST /session/token.
type TokenResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	UserName    string `json:"user_name"`
	APIKey      string `json:"api_key"`
	LoginTime   string `json:"login_time"`
}

type apiResponse[T any] struct {
	Status  string `json:"status"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}
