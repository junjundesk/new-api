package controller

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupEmailAdminDB swaps in an in-memory DB holding just the users table.
func setupEmailAdminDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	return db
}

func seedEmailUser(t *testing.T, db *gorm.DB, id int, email string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: "email-user-" + strconv.Itoa(id),
		Email:    email,
		AffCode:  "af" + strconv.Itoa(id),
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
}

// TestAdminSendEmailRecipientsSkipsMissingAddresses covers the batch contract:
// users without a usable email are counted as skipped, never attempted, and do
// not abort the rest of the batch.
func TestAdminSendEmailRecipientsSkipsMissingAddresses(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9001, "a@example.com")
	seedEmailUser(t, db, 9002, "")
	seedEmailUser(t, db, 9003, "   ")
	seedEmailUser(t, db, 9004, "d@example.com")

	recipients, skipped, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds: []int{9001, 9002, 9003, 9004},
	})
	require.NoError(t, err)
	assert.Len(t, recipients, 2, "only users with a usable address are recipients")
	assert.Equal(t, 2, skipped, "blank addresses are reported, not silently dropped")

	for _, r := range recipients {
		assert.NotEmpty(t, r.Email)
		assert.NotContains(t, []int{9002, 9003}, r.Id)
	}
}

// TestAdminSendEmailRecipientsDeduplicatesIds guards against a caller passing a
// repeated id and the user receiving the same mail twice.
func TestAdminSendEmailRecipientsDeduplicatesIds(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9101, "dup@example.com")

	recipients, _, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds: []int{9101, 9101, 9101},
	})
	require.NoError(t, err)
	assert.Len(t, recipients, 1)
}

// TestAdminSendEmailRecipientsIgnoresUnknownIds keeps a stale selection from
// failing the whole send when a user was deleted meanwhile.
func TestAdminSendEmailRecipientsIgnoresUnknownIds(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9201, "known@example.com")

	recipients, _, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds: []int{9201, 999999},
	})
	require.NoError(t, err)
	require.Len(t, recipients, 1)
	assert.Equal(t, 9201, recipients[0].Id)
}

// TestAdminSendEmailRecipientsAllUsersTargetsEveryone verifies the broadcast
// path selects all users rather than requiring an id list.
func TestAdminSendEmailRecipientsAllUsersTargetsEveryone(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9301, "one@example.com")
	seedEmailUser(t, db, 9302, "two@example.com")
	seedEmailUser(t, db, 9303, "")

	recipients, skipped, err := adminSendEmailRecipients(&AdminSendEmailRequest{AllUsers: true})
	require.NoError(t, err)
	assert.Len(t, recipients, 2)
	assert.Equal(t, 1, skipped)
}
