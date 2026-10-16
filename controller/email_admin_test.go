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
	seedEmailUserWithSetting(t, db, id, email, "")
}

// seedEmailUserWithSetting seeds a user whose stored settings JSON carries an
// operational-notification preference, as persisted in users.setting.
func seedEmailUserWithSetting(t *testing.T, db *gorm.DB, id int, email string, setting string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: "email-user-" + strconv.Itoa(id),
		Email:    email,
		AffCode:  "af" + strconv.Itoa(id),
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Setting:  setting,
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

	recipients, skippedNoEmail, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds: []int{9001, 9002, 9003, 9004},
	})
	require.NoError(t, err)
	assert.Len(t, recipients, 2, "only users with a usable address are recipients")
	assert.Equal(t, 2, skippedNoEmail, "blank addresses are reported, not silently dropped")
	assert.Zero(t, skippedUnsubscribed)

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

	recipients, _, _, err := adminSendEmailRecipients(&AdminSendEmailRequest{
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

	recipients, _, _, err := adminSendEmailRecipients(&AdminSendEmailRequest{
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

	recipients, skippedNoEmail, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{AllUsers: true})
	require.NoError(t, err)
	assert.Len(t, recipients, 2)
	assert.Equal(t, 1, skippedNoEmail)
	assert.Zero(t, skippedUnsubscribed)
}

// recipientIds extracts the selected ids for order-independent assertions.
func recipientIds(recipients []model.User) []int {
	ids := make([]int, 0, len(recipients))
	for _, recipient := range recipients {
		ids = append(ids, recipient.Id)
	}
	return ids
}

// TestAdminSendEmailRecipientsSkipsUsersWhoTurnedOffOperationalNotifications
// covers the opt-out contract: a user who disabled operational notifications is
// reported separately and never receives a campaign.
func TestAdminSendEmailRecipientsSkipsUsersWhoTurnedOffOperationalNotifications(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9401, "unset@example.com")
	seedEmailUserWithSetting(t, db, 9402, "opted-out@example.com", `{"operational_notify_enabled":false}`)
	seedEmailUserWithSetting(t, db, 9403, "opted-in@example.com", `{"operational_notify_enabled":true}`)

	recipients, skippedNoEmail, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds: []int{9401, 9402, 9403},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{9401, 9403}, recipientIds(recipients))
	assert.Zero(t, skippedNoEmail)
	assert.Equal(t, 1, skippedUnsubscribed, "the opting-out user is counted, not silently dropped")
}

// TestAdminSendEmailRecipientsAllUsersFiltersUnsubscribed keeps the broadcast
// path honest: the all-users query must load the setting column, otherwise the
// opt-out filter would silently stop working.
func TestAdminSendEmailRecipientsAllUsersFiltersUnsubscribed(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUser(t, db, 9501, "subscribed@example.com")
	seedEmailUserWithSetting(t, db, 9502, "opted-out@example.com", `{"operational_notify_enabled":false}`)

	recipients, skippedNoEmail, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{AllUsers: true})
	require.NoError(t, err)
	assert.Equal(t, []int{9501}, recipientIds(recipients))
	assert.Zero(t, skippedNoEmail)
	assert.Equal(t, 1, skippedUnsubscribed)
}

// TestAdminSendEmailRecipientsAllowsAdminOverride verifies the deliberate
// override: an admin who unchecks the filter may still reach every user.
func TestAdminSendEmailRecipientsAllowsAdminOverride(t *testing.T) {
	db := setupEmailAdminDB(t)
	seedEmailUserWithSetting(t, db, 9601, "opted-out@example.com", `{"operational_notify_enabled":false}`)

	recipients, skippedNoEmail, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{
		UserIds:             []int{9601},
		IncludeUnsubscribed: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{9601}, recipientIds(recipients))
	assert.Zero(t, skippedNoEmail)
	assert.Zero(t, skippedUnsubscribed)
}

// TestAdminSendEmailRecipientsTreatsUnsetSettingAsSubscribed pins the site
// default: an absent, empty or unparsable setting means operational mail is on.
func TestAdminSendEmailRecipientsTreatsUnsetSettingAsSubscribed(t *testing.T) {
	db := setupEmailAdminDB(t)
	settings := []string{"", "{}", "{not-json"}

	for index, setting := range settings {
		id := 9701 + index
		seedEmailUserWithSetting(t, db, id, "user@example.com", setting)

		recipients, _, skippedUnsubscribed, err := adminSendEmailRecipients(&AdminSendEmailRequest{
			UserIds: []int{id},
		})
		require.NoError(t, err)
		assert.Equal(t, []int{id}, recipientIds(recipients), "setting %q keeps the user subscribed", setting)
		assert.Zero(t, skippedUnsubscribed)
	}
}
