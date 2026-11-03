package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBaseURLRouter(now time.Time) (*baseURLRouter, *time.Time) {
	clock := now
	router := newBaseURLRouter()
	router.now = func() time.Time { return clock }
	return router, &clock
}

func newTestChannel(id int, urls ...string) *model.Channel {
	primary := ""
	if len(urls) > 0 {
		primary = urls[0]
	}
	return &model.Channel{Id: id, BaseURL: &primary, BaseURLs: urls}
}

func TestSelectChannelBaseURLSingleURL(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())

	got := router.selectBaseURL(newTestChannel(1, "https://only.example"))

	assert.Equal(t, "https://only.example", got)
	assert.Empty(t, router.routes)
}

func TestSelectChannelBaseURLColdStartRoundRobin(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())
	channel := newTestChannel(1, "https://a.example", "https://b.example", "https://c.example")

	seen := map[string]int{}
	for i := 0; i < 3; i++ {
		url := router.selectBaseURL(channel)
		require.Contains(t, channel.BaseURLs, url)
		seen[url]++
	}

	assert.Len(t, seen, 3)
}

func TestSelectChannelBaseURLPrefersFastestSample(t *testing.T) {
	now := time.Now()
	router, clock := newTestBaseURLRouter(now)
	channel := newTestChannel(1, "https://a.example", "https://b.example", "https://c.example")

	router.selectBaseURL(channel)
	router.reportSuccess(1, "https://a.example", 100*time.Millisecond)
	router.reportSuccess(1, "https://b.example", 50*time.Millisecond)
	router.reportSuccess(1, "https://c.example", 300*time.Millisecond)
	*clock = now.Add(baseURLStickyTTL + time.Second)

	assert.Equal(t, "https://b.example", router.selectBaseURL(channel))
}

func TestSelectChannelBaseURLSkipsCoolingURL(t *testing.T) {
	now := time.Now()
	router, clock := newTestBaseURLRouter(now)
	channel := newTestChannel(1, "https://a.example", "https://b.example")

	router.selectBaseURL(channel)
	router.reportSuccess(1, "https://a.example", 50*time.Millisecond)
	router.reportSuccess(1, "https://b.example", 300*time.Millisecond)
	*clock = now.Add(baseURLStickyTTL + time.Second)
	router.reportFailure(1, "https://a.example")

	assert.Equal(t, "https://b.example", router.selectBaseURL(channel))
}

func TestSelectChannelBaseURLFallsBackWhenAllCooling(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())
	channel := newTestChannel(1, "https://a.example", "https://b.example")

	router.selectBaseURL(channel)
	router.reportFailure(1, "https://a.example")
	router.reportFailure(1, "https://b.example")

	assert.Contains(t, channel.BaseURLs, router.selectBaseURL(channel))
}

func TestReportBaseURLSuccessUpdatesEWMA(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())
	channel := newTestChannel(1, "https://a.example", "https://b.example")

	router.selectBaseURL(channel)
	router.reportSuccess(1, "https://a.example", 100*time.Millisecond)
	router.reportSuccess(1, "https://a.example", 200*time.Millisecond)

	stat := router.routes[1].stats["https://a.example"]
	assert.InDelta(t, float64(130*time.Millisecond), float64(stat.ttfbEWMA), float64(time.Millisecond))
	assert.Equal(t, 2, stat.samples)
}

func TestReportBaseURLFailureBackoffAndReset(t *testing.T) {
	now := time.Now()
	router, _ := newTestBaseURLRouter(now)
	channel := newTestChannel(1, "https://a.example", "https://b.example")

	router.selectBaseURL(channel)
	for i := 0; i < 5; i++ {
		router.reportFailure(1, "https://a.example")
	}
	stat := router.routes[1].stats["https://a.example"]
	assert.Equal(t, baseURLCooldownMax, stat.cooldownUntil.Sub(now))

	router.reportSuccess(1, "https://a.example", 10*time.Millisecond)
	assert.Zero(t, stat.consecutiveFails)
	assert.True(t, stat.cooldownUntil.IsZero())

	router.reportFailure(1, "https://a.example")
	assert.Equal(t, baseURLCooldownBase, stat.cooldownUntil.Sub(now))
}

func TestNextBaseURLSkipsTriedAndCooling(t *testing.T) {
	now := time.Now()
	router, _ := newTestBaseURLRouter(now)
	channel := newTestChannel(1, "https://a.example", "https://b.example", "https://c.example")

	router.selectBaseURL(channel)
	router.reportSuccess(1, "https://b.example", 50*time.Millisecond)
	router.reportSuccess(1, "https://c.example", 80*time.Millisecond)
	router.reportFailure(1, "https://a.example")

	next, ok := router.nextBaseURL(1, "https://a.example", []string{"https://a.example"})
	require.True(t, ok)
	assert.Equal(t, "https://b.example", next)

	next, ok = router.nextBaseURL(1, "https://a.example", []string{"https://a.example", "https://b.example"})
	require.True(t, ok)
	assert.Equal(t, "https://c.example", next)

	_, ok = router.nextBaseURL(1, "https://c.example", []string{"https://a.example", "https://b.example", "https://c.example"})
	assert.False(t, ok)

	_, ok = router.nextBaseURL(2, "https://a.example", nil)
	assert.False(t, ok)
}

func TestBaseURLReportsDoNotCreateRoute(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())

	router.reportSuccess(7, "https://a.example", time.Second)
	router.reportFailure(7, "https://a.example")
	assert.Empty(t, router.routes)

	channel := newTestChannel(7, "https://a.example", "https://b.example")
	router.selectBaseURL(channel)
	router.invalidateChannelBaseURLs(7)
	router.reportSuccess(7, "https://a.example", time.Second)
	assert.Empty(t, router.routes)
}

func TestSelectChannelBaseURLRebuildsRouteWhenListChanges(t *testing.T) {
	router, _ := newTestBaseURLRouter(time.Now())

	router.selectBaseURL(newTestChannel(1, "https://a.example", "https://b.example"))
	router.reportSuccess(1, "https://a.example", 20*time.Millisecond)
	require.True(t, router.routes[1].stats["https://a.example"].hasTTFB)

	router.selectBaseURL(newTestChannel(1, "https://c.example", "https://d.example"))
	route := router.routes[1]
	assert.Equal(t, []string{"https://c.example", "https://d.example"}, route.urls)
	assert.False(t, route.stats["https://c.example"].hasTTFB)
}
