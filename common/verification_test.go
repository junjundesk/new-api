package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyCodeWithKeyConsumesCodeOnSuccess(t *testing.T) {
	key := "consume@example.com"
	RegisterVerificationCodeWithKey(key, "123456", EmailVerificationPurpose)

	require.True(t, VerifyCodeWithKey(key, "123456", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey(key, "123456", EmailVerificationPurpose))
}

func TestVerifyCodeWithKeyLocksOutAfterMaxAttempts(t *testing.T) {
	key := "lockout@example.com"
	RegisterVerificationCodeWithKey(key, "123456", EmailVerificationPurpose)

	for i := 0; i < verificationMaxAttempts; i++ {
		require.False(t, VerifyCodeWithKey(key, "000000", EmailVerificationPurpose))
	}
	assert.False(t, VerifyCodeWithKey(key, "123456", EmailVerificationPurpose))
}

func TestVerifyCodeWithKeySeparatesPurposes(t *testing.T) {
	key := "purpose@example.com"
	RegisterVerificationCodeWithKey(key, "123456", PasswordResetPurpose)

	assert.False(t, VerifyCodeWithKey(key, "123456", EmailVerificationPurpose))
	assert.True(t, VerifyCodeWithKey(key, "123456", PasswordResetPurpose))
}
