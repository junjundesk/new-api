package controller

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelBaseURLs(t *testing.T) {
	primary := "https://primary.example/"
	normalized, err := normalizeChannelBaseURLs(nil, &primary)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://primary.example"}, normalized)

	normalized, err = normalizeChannelBaseURLs([]string{
		" https://a.example/ ",
		"",
		"https://a.example",
		"https://b.example///",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, normalized)

	enough := make([]string, maxChannelBaseURLs)
	for i := range enough {
		enough[i] = fmt.Sprintf("https://h%d.example", i)
	}
	normalized, err = normalizeChannelBaseURLs(enough, nil)
	require.NoError(t, err)
	assert.Len(t, normalized, maxChannelBaseURLs)

	tooMany := append(enough, "https://overflow.example")
	_, err = normalizeChannelBaseURLs(tooMany, nil)
	assert.Error(t, err)

	for _, invalid := range []string{"ftp://a.example", "a.example", "/relative", "https://"} {
		_, err = normalizeChannelBaseURLs([]string{invalid}, nil)
		assert.Error(t, err, invalid)
	}
}

func TestApplyChannelBaseURLs(t *testing.T) {
	channel := &model.Channel{Id: 1}
	require.NoError(t, applyChannelBaseURLs(channel, []string{"https://a.example/", "https://b.example"}, nil))
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, channel.BaseURLs)
	require.NotNil(t, channel.BaseURL)
	assert.Equal(t, "https://a.example", *channel.BaseURL)

	require.NoError(t, applyChannelBaseURLs(channel, nil, nil))
	assert.Empty(t, channel.BaseURLs)
	require.NotNil(t, channel.BaseURL)
	assert.Empty(t, *channel.BaseURL)
}
