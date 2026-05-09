package tools

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const boundUser = "11111111-1111-1111-1111-111111111111"

func TestGetAccountSummary_FallsBackToBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return(nil, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No accounts found.", got)
}

func TestGetAccountSummary_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), "other-user").Return(nil, nil)

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{"user_id":"other-user"}`)
	require.NoError(t, err)
}

func TestGetAccountSummary_FormatsActiveAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:           id,
		Name:         "HDFC Savings",
		Institution:  "hdfc",
		AccountType:  sqlcgen.AccountTypeEnumBankSavings,
		AccountClass: sqlcgen.AccountClassEnumAsset,
		IsActive:     true,
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "HDFC Savings")
	assert.Contains(t, got, "hdfc")
	assert.Contains(t, got, "id:"+id.String())
	assert.NotContains(t, got, "[inactive]")
}

func TestGetAccountSummary_FormatsInactiveAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:           uuid.New(),
		Name:         "Old CC",
		Institution:  "citi",
		AccountType:  sqlcgen.AccountTypeEnumCreditCard,
		AccountClass: sqlcgen.AccountClassEnumLiability,
		IsActive:     false,
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "[inactive]")
}

func TestGetAccountSummary_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return(nil, errors.New("db down"))

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestGetAccountSummary_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `not json`)
	require.Error(t, err)
}
