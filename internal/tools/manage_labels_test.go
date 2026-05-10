package tools

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestManageLabels_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	txnID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	lblID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	q.EXPECT().AddToTransaction(gomock.Any(), txnID, lblID).Return(nil)

	got, err := tool.Execute(t.Context(), "", `{"action":"add","transaction_id":"`+txnID+`","label_id":"`+lblID+`"}`)
	require.NoError(t, err)
	assert.Contains(t, got, lblID)
	assert.Contains(t, got, "added")
}

func TestManageLabels_Remove(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	txnID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	lblID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	q.EXPECT().RemoveFromTransaction(gomock.Any(), txnID, lblID).Return(nil)

	got, err := tool.Execute(t.Context(), "", `{"action":"remove","transaction_id":"`+txnID+`","label_id":"`+lblID+`"}`)
	require.NoError(t, err)
	assert.Contains(t, got, lblID)
	assert.Contains(t, got, "removed")
}

func TestManageLabels_UnknownAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	_, err := tool.Execute(t.Context(), "", `{"action":"upsert","transaction_id":"a","label_id":"b"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestManageLabels_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	txnID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	lblID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	q.EXPECT().AddToTransaction(gomock.Any(), txnID, lblID).Return(errors.New("db error"))

	_, err := tool.Execute(t.Context(), "", `{"action":"add","transaction_id":"`+txnID+`","label_id":"`+lblID+`"}`)
	require.Error(t, err)
}

func TestManageLabels_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	_, err := tool.Execute(t.Context(), "", `not-json`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse args")
}

func TestManageLabels_RemoveStoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocklabelQuerier(ctrl)
	tool := NewManageLabels(q)

	txnID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	lblID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	q.EXPECT().RemoveFromTransaction(gomock.Any(), txnID, lblID).Return(errors.New("db error"))

	_, err := tool.Execute(t.Context(), "", `{"action":"remove","transaction_id":"`+txnID+`","label_id":"`+lblID+`"}`)
	require.Error(t, err)
}

func TestManageLabels_Definition(t *testing.T) {
	q := NewMocklabelQuerier(gomock.NewController(t))
	def := NewManageLabels(q).Definition()
	assert.Equal(t, "manage_labels", def.Name)
}
