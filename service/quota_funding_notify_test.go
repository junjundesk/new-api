package service

import (
	"fmt"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderNotify substitutes a notification's values into its content, the same
// way sendEmailNotify does.
func renderNotify(n dto.Notify) string {
	rendered := n.Content
	for _, value := range n.Values {
		rendered = strings.Replace(rendered, dto.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}
	return rendered
}

// TestBuildFundingQuotaNotifyReportsBucketNotWallet guards the regression where
// a bonus-billed request warned the user their remaining quota was zero: the
// wallet path quotes relayInfo.UserQuota, which is untouched when the charge
// comes out of a limited bucket.
func TestBuildFundingQuotaNotifyReportsBucketNotWallet(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		UserId:    42,
		UserEmail: "user@example.com",
		// Wallet is empty — the request was paid from bonus.
		UserQuota: 0,
		UserSetting: dto.UserSetting{
			NotifyType: dto.NotifyTypeEmail,
		},
	}

	notification := buildFundingQuotaNotify(relayInfo, 12345, "您的赠金额度即将用尽", "赠金")
	require.Len(t, notification.Values, 4, "email template needs prompt, amount and two link slots")

	rendered := renderNotify(notification)
	assert.Contains(t, rendered, "赠金", "the warning must name the funding bucket")
	assert.NotContains(t, rendered, "0.000000",
		"the wallet balance must not leak into a bonus-billed warning")
	assert.NotContains(t, rendered, dto.ContentValueParam, "all placeholders must be filled")
	assert.Contains(t, notification.Title, "赠金额度")
}

// TestBuildFundingQuotaNotifyBarkStaysPlainText covers the reduced-content
// channel, which rejects HTML.
func TestBuildFundingQuotaNotifyBarkStaysPlainText(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		UserId:      43,
		UserSetting: dto.UserSetting{NotifyType: dto.NotifyTypeBark},
	}

	notification := buildFundingQuotaNotify(relayInfo, 1, "您的订阅额度即将用尽", "额度")
	assert.Len(t, notification.Values, 2)
	assert.NotContains(t, notification.Content, "<a href")
}

// TestBuildFundingQuotaNotifyDefaultChannelIsHtml checks the empty notify type
// falls back to the HTML-capable content.
func TestBuildFundingQuotaNotifyDefaultChannelIsHtml(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{UserId: 44}

	notification := buildFundingQuotaNotify(relayInfo, 1, "您的赠金额度即将用尽", "赠金")
	assert.Contains(t, notification.Content, "<a href")
}
