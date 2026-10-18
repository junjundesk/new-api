package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// User token creation errors are exported so callers can map them to their own
// localized responses instead of surfacing raw validation text.
var (
	// ErrTokenNameTooLong reports a token name longer than the supported limit.
	ErrTokenNameTooLong = errors.New("token name is too long")
	// ErrTokenNameInvalid reports an empty token name or one with unsupported characters.
	ErrTokenNameInvalid = errors.New("token name contains invalid characters")
	// ErrTokenQuotaNegative reports a negative quota on a finite token.
	ErrTokenQuotaNegative = errors.New("token quota cannot be negative")
	// ErrTokenQuotaExceedMax reports a finite quota above the supported maximum.
	ErrTokenQuotaExceedMax = errors.New("token quota exceeds the maximum")
	// ErrTokenLimitReached reports that the user already owns the maximum number of tokens.
	ErrTokenLimitReached = errors.New("user token count limit reached")
	// ErrTokenKeyGenerateFailed reports that a random token key could not be generated.
	ErrTokenKeyGenerateFailed = errors.New("failed to generate token key")
)

const (
	// userTokenNameMaxLengthBytes matches the historical controller guard, which
	// rejected names longer than 50 bytes before checking the character set.
	userTokenNameMaxLengthBytes = 50
	// userTokenQuotaMultiplier bounds a finite token quota relative to
	// common.QuotaPerUnit (1000000000 units by default).
	userTokenQuotaMultiplier = 1000000000
)

var userTokenNamePattern = regexp.MustCompile(`^[\p{L}\p{N} _\-\.\(\)\[\]@#+]{1,50}$`)

// MaxUserTokenQuota returns the largest quota a finite user token may hold.
func MaxUserTokenQuota() int {
	return int(userTokenQuotaMultiplier * common.QuotaPerUnit)
}

// NormalizeUserTokenName trims and validates a user token name. It returns the
// normalized name on success and ErrTokenNameTooLong / ErrTokenNameInvalid on
// failure, preserving the order the HTTP token API has always used.
func NormalizeUserTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len(name) > userTokenNameMaxLengthBytes {
		return "", ErrTokenNameTooLong
	}
	if name == "" || !userTokenNamePattern.MatchString(name) {
		return "", ErrTokenNameInvalid
	}
	return name, nil
}

// ValidateUserTokenQuota rejects a finite token quota outside the supported
// range. Unlimited tokens skip the bound because the value is never charged.
func ValidateUserTokenQuota(remainQuota int, unlimited bool) error {
	if unlimited {
		return nil
	}
	if remainQuota < 0 {
		return ErrTokenQuotaNegative
	}
	if remainQuota > MaxUserTokenQuota() {
		return ErrTokenQuotaExceedMax
	}
	return nil
}

// InsertUserToken validates and persists a user-owned token: name
// normalization, quota bounds, the per-user token count limit, key generation
// and creation timestamps all live here so every creator produces equivalent
// tokens. Token routing fields (group, auto groups, model mappings) are the
// caller's responsibility and are persisted untouched.
func InsertUserToken(userID int, token *model.Token) error {
	if token == nil {
		return errors.New("token is required")
	}
	name, err := NormalizeUserTokenName(token.Name)
	if err != nil {
		return err
	}
	if err := ValidateUserTokenQuota(token.RemainQuota, token.UnlimitedQuota); err != nil {
		return err
	}
	count, err := model.CountUserTokens(userID)
	if err != nil {
		return err
	}
	if int(count) >= operation_setting.GetMaxUserTokens() {
		return ErrTokenLimitReached
	}
	key, err := common.GenerateKey()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTokenKeyGenerateFailed, err)
	}

	now := common.GetTimestamp()
	token.UserId = userID
	token.Name = name
	token.Key = key
	token.CreatedTime = now
	token.AccessedTime = now
	token.Status = common.TokenStatusEnabled
	return token.Insert()
}
