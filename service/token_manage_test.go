package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserTokenForTest(name string, remainQuota int) *model.Token {
	return &model.Token{
		Name:        name,
		RemainQuota: remainQuota,
		ExpiredTime: -1,
	}
}

func TestNormalizeUserTokenName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "plain", input: "my-key", want: "my-key"},
		{name: "trimmed", input: "  my key  ", want: "my key"},
		{name: "unicode and symbols", input: "密钥-01 (test)[x]@#+", want: "密钥-01 (test)[x]@#+"},
		{name: "exactly 50 bytes", input: strings.Repeat("a", 50), want: strings.Repeat("a", 50)},
		{name: "over 50 bytes", input: strings.Repeat("a", 51), wantErr: ErrTokenNameTooLong},
		{name: "empty", input: "", wantErr: ErrTokenNameInvalid},
		{name: "blank", input: "   ", wantErr: ErrTokenNameInvalid},
		{name: "slash", input: "bad/name", wantErr: ErrTokenNameInvalid},
		{name: "newline", input: "bad\nname", wantErr: ErrTokenNameInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeUserTokenName(tc.input)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateUserTokenQuota(t *testing.T) {
	maxQuota := MaxUserTokenQuota()
	require.Positive(t, maxQuota)

	cases := []struct {
		name      string
		remain    int
		unlimited bool
		wantErr   error
	}{
		{name: "zero finite", remain: 0},
		{name: "maximum finite", remain: maxQuota},
		{name: "above maximum finite", remain: maxQuota + 1, wantErr: ErrTokenQuotaExceedMax},
		{name: "negative finite", remain: -1, wantErr: ErrTokenQuotaNegative},
		{name: "negative unlimited", remain: -1, unlimited: true},
		{name: "above maximum unlimited", remain: maxQuota + 1, unlimited: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUserTokenQuota(tc.remain, tc.unlimited)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInsertUserTokenPersistsNormalizedToken(t *testing.T) {
	truncate(t)

	token := newUserTokenForTest("  my-token  ", 120)
	token.UnlimitedQuota = false
	token.Group = "default"
	token.AutoGroups = `["vip"]`
	require.NoError(t, InsertUserToken(7, token))

	assert.NotZero(t, token.Id)
	assert.Equal(t, 7, token.UserId)
	assert.Equal(t, "my-token", token.Name)
	assert.Len(t, token.Key, 48)
	assert.Equal(t, common.TokenStatusEnabled, token.Status)
	assert.NotZero(t, token.CreatedTime)
	assert.Equal(t, token.CreatedTime, token.AccessedTime)
	assert.Equal(t, 120, token.RemainQuota)
	assert.Equal(t, "default", token.Group)
	assert.Equal(t, `["vip"]`, token.AutoGroups)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	assert.Equal(t, 7, stored.UserId)
	assert.Equal(t, "my-token", stored.Name)
	assert.Equal(t, token.Key, stored.Key)
	assert.Equal(t, common.TokenStatusEnabled, stored.Status)
	assert.Equal(t, `["vip"]`, stored.AutoGroups)
}

func TestInsertUserTokenRejectsInvalidInputWithoutPersisting(t *testing.T) {
	cases := []struct {
		name    string
		token   *model.Token
		wantErr error
	}{
		{name: "invalid name", token: newUserTokenForTest("bad/name", 10), wantErr: ErrTokenNameInvalid},
		{name: "too long name", token: newUserTokenForTest(strings.Repeat("a", 51), 10), wantErr: ErrTokenNameTooLong},
		{name: "negative quota", token: newUserTokenForTest("ok-name", -1), wantErr: ErrTokenQuotaNegative},
		{name: "quota above maximum", token: newUserTokenForTest("ok-name", MaxUserTokenQuota()+1), wantErr: ErrTokenQuotaExceedMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)

			err := InsertUserToken(7, tc.token)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Zero(t, tc.token.Id)
			assert.Empty(t, tc.token.Key)

			count, countErr := model.CountUserTokens(7)
			require.NoError(t, countErr)
			assert.Zero(t, count)
		})
	}
}

func TestInsertUserTokenEnforcesMaxUserTokensLimit(t *testing.T) {
	truncate(t)

	tokenSetting := operation_setting.GetTokenSetting()
	oldMaxTokens := tokenSetting.MaxUserTokens
	tokenSetting.MaxUserTokens = 1
	t.Cleanup(func() { tokenSetting.MaxUserTokens = oldMaxTokens })

	require.NoError(t, InsertUserToken(7, newUserTokenForTest("first", 10)))

	err := InsertUserToken(7, newUserTokenForTest("second", 10))
	require.ErrorIs(t, err, ErrTokenLimitReached)

	count, countErr := model.CountUserTokens(7)
	require.NoError(t, countErr)
	assert.Equal(t, int64(1), count)

	// The limit is per user, so another user is unaffected.
	require.NoError(t, InsertUserToken(8, newUserTokenForTest("other-user", 10)))
}
