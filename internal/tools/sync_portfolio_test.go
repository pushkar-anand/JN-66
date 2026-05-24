package tools

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/pushkaranand/finagent/internal/store"
)

func TestSyncPortfolio_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockzerodhaSyncer(ctrl)
	q.EXPECT().ForceSync(gomock.Any(), uuid.MustParse(boundUser)).Return(12, 5, nil)
	got, err := NewSyncPortfolio(boundUser, q, nil).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "Synced 12 equity holdings and 5 mutual fund holdings.", got)
}

func TestSyncPortfolio_TokenExpired_NoURLFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockzerodhaSyncer(ctrl)
	q.EXPECT().ForceSync(gomock.Any(), uuid.MustParse(boundUser)).Return(0, 0, store.ErrZerodhaTokenExpired)
	got, err := NewSyncPortfolio(boundUser, q, nil).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "finagent zerodha auth")
}

func TestSyncPortfolio_TokenExpired_WithURLFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockzerodhaSyncer(ctrl)
	q.EXPECT().ForceSync(gomock.Any(), uuid.MustParse(boundUser)).Return(0, 0, store.ErrZerodhaTokenExpired)
	got, err := NewSyncPortfolio(boundUser, q, func() string { return "https://kite.example/login" }).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "https://kite.example/login")
	assert.Contains(t, got, "logged in")
}

func TestSyncPortfolio_APIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockzerodhaSyncer(ctrl)
	q.EXPECT().ForceSync(gomock.Any(), uuid.MustParse(boundUser)).Return(0, 0, errors.New("kite api down"))
	_, err := NewSyncPortfolio(boundUser, q, nil).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestSyncPortfolio_InvalidUserID(t *testing.T) {
	q := NewMockzerodhaSyncer(gomock.NewController(t))
	_, err := NewSyncPortfolio("not-a-uuid", q, nil).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestSyncPortfolio_Definition(t *testing.T) {
	def := NewSyncPortfolio(boundUser, NewMockzerodhaSyncer(gomock.NewController(t)), nil).Definition()
	assert.Equal(t, "sync_portfolio", def.Name)
}
