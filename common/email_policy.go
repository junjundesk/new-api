package common

import (
	"errors"
	"strings"
)

// QQMailDomain is the Tencent mail domain whose local part is an alias string
// rather than a fixed mailbox name.
const QQMailDomain = "qq.com"

// ErrQQEmailNotNumeric is returned for a qq.com address whose local part is not
// a pure QQ number.
var ErrQQEmailNotNumeric = errors.New("qq.com email address must use a pure numeric QQ number as the mailbox name")

// ParseEmailDomainWhitelist splits the stored comma-separated whitelist into
// trimmed, de-duplicated, lower-cased entries. Blank entries are dropped so a
// trailing comma cannot turn into a domain that matches nothing meaningful.
func ParseEmailDomainWhitelist(value string) []string {
	parts := strings.Split(value, ",")
	domains := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		domain := strings.ToLower(strings.TrimSpace(part))
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	return domains
}

// IsQQNumericRuleEnabled reports whether qq.com is listed in the active email
// domain whitelist. Listing the domain is the operator's signal that QQ
// registrations are expected to be real QQ numbers; when the whitelist is
// disabled the whitelist entry is inert, so the rule stays inert too.
func IsQQNumericRuleEnabled() bool {
	if !EmailDomainRestrictionEnabled {
		return false
	}
	for _, domain := range EmailDomainWhitelist {
		if strings.EqualFold(strings.TrimSpace(domain), QQMailDomain) {
			return true
		}
	}
	return false
}

// ValidateQQEmailLocalPart rejects qq.com addresses that are not a pure QQ
// number.
//
// Tencent treats the mailbox name of a qq.com address as an alias, so without
// this check a single QQ account can register unlimited site accounts
// (aa@qq.com, bb@qq.com, ...). Addresses on other domains, empty input, and
// malformed input are left to the regular format validation.
func ValidateQQEmailLocalPart(email string) error {
	if !IsQQNumericRuleEnabled() {
		return nil
	}

	trimmed := strings.TrimSpace(email)
	at := strings.LastIndex(trimmed, "@")
	if at < 0 {
		return nil
	}
	localPart := trimmed[:at]
	domain := strings.ToLower(trimmed[at+1:])
	if domain != QQMailDomain {
		return nil
	}
	if localPart == "" {
		return ErrQQEmailNotNumeric
	}
	for _, r := range localPart {
		if r < '0' || r > '9' {
			return ErrQQEmailNotNumeric
		}
	}
	return nil
}
