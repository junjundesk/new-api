package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withQQWhitelist runs a test with the domain whitelist enabled and restored
// afterwards, so the global option state does not leak between tests.
func withQQWhitelist(t *testing.T, enabled bool, domains []string) {
	t.Helper()
	prevEnabled := EmailDomainRestrictionEnabled
	prevList := EmailDomainWhitelist
	EmailDomainRestrictionEnabled = enabled
	EmailDomainWhitelist = domains
	t.Cleanup(func() {
		EmailDomainRestrictionEnabled = prevEnabled
		EmailDomainWhitelist = prevList
	})
}

// TestValidateQQEmailLocalPartRejectsAliases covers the rule's purpose: Tencent
// treats a qq.com mailbox name as an alias, so a non-numeric name lets one QQ
// account register many site accounts.
func TestValidateQQEmailLocalPartRejectsAliases(t *testing.T) {
	withQQWhitelist(t, true, []string{"gmail.com", "qq.com"})

	for _, email := range []string{
		"ferfnj@qq.com",
		"abc123@qq.com",
		"a.b@qq.com",
		"user+tag@qq.com",
		"@qq.com",
		"１２３@qq.com", // full-width digits are not a valid QQ number
	} {
		assert.ErrorIsf(t, ValidateQQEmailLocalPart(email), ErrQQEmailNotNumeric,
			"%q must be rejected", email)
	}
}

func TestValidateQQEmailLocalPartAcceptsNumeric(t *testing.T) {
	withQQWhitelist(t, true, []string{"qq.com"})

	for _, email := range []string{
		"123456@qq.com",
		"10001@qq.com",
		"123456@QQ.com",
		"  123456@qq.com  ",
	} {
		assert.NoErrorf(t, ValidateQQEmailLocalPart(email), "%q must be accepted", email)
	}
}

// TestValidateQQEmailLocalPartIgnoresOtherDomains keeps the rule scoped: other
// whitelisted domains keep accepting ordinary names.
func TestValidateQQEmailLocalPartIgnoresOtherDomains(t *testing.T) {
	withQQWhitelist(t, true, []string{"qq.com", "gmail.com"})

	assert.NoError(t, ValidateQQEmailLocalPart("ferfnj@gmail.com"))
	assert.NoError(t, ValidateQQEmailLocalPart("first.last@163.com"))
	// A domain that merely contains "qq.com" must not be treated as QQ mail.
	assert.NoError(t, ValidateQQEmailLocalPart("someone@notqq.com"))
}

// TestValidateQQEmailLocalPartOnlyAppliesWhenWhitelistActive ties the rule to
// the operator's whitelist configuration: with restriction off, or qq.com not
// listed, QQ aliases stay allowed.
func TestValidateQQEmailLocalPartOnlyAppliesWhenWhitelistActive(t *testing.T) {
	withQQWhitelist(t, false, []string{"qq.com"})
	assert.NoError(t, ValidateQQEmailLocalPart("ferfnj@qq.com"),
		"rule must stay inert while the whitelist is disabled")

	withQQWhitelist(t, true, []string{"gmail.com", "163.com"})
	assert.NoError(t, ValidateQQEmailLocalPart("ferfnj@qq.com"),
		"rule must not apply when qq.com is absent from the whitelist")
}

// TestValidateQQEmailLocalPartLeavesMalformedInputAlone defers format errors to
// the existing email validator instead of reporting them as a QQ-number problem.
func TestValidateQQEmailLocalPartLeavesMalformedInputAlone(t *testing.T) {
	withQQWhitelist(t, true, []string{"qq.com"})

	assert.NoError(t, ValidateQQEmailLocalPart(""))
	assert.NoError(t, ValidateQQEmailLocalPart("no-at-sign"))
}

// TestValidateQQEmailLocalPartMatchesWhitelistWithSpaces guards against the
// whitelist entry being written with stray whitespace or different casing.
func TestValidateQQEmailLocalPartMatchesWhitelistWithSpaces(t *testing.T) {
	withQQWhitelist(t, true, []string{" qq.com ", "QQ.COM"})
	assert.ErrorIs(t, ValidateQQEmailLocalPart("ferfnj@qq.com"), ErrQQEmailNotNumeric)
}

func TestParseEmailDomainWhitelist(t *testing.T) {
	got := ParseEmailDomainWhitelist(" gmail.com , QQ.com,,163.com ,gmail.com")
	require.Equal(t, []string{"gmail.com", "qq.com", "163.com"}, got,
		"entries are trimmed, lower-cased, de-duplicated and blanks dropped")

	assert.Empty(t, ParseEmailDomainWhitelist(""))
	assert.Empty(t, ParseEmailDomainWhitelist(",,,"))
}
