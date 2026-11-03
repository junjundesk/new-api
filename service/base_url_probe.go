package service

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	baseURLProbeTimeout = 3 * time.Second
	baseURLProbeWorkers = 8
)

// StartBaseURLProbeTask 定期探测已注册渠道的地址可达性，为没有真实请求样本的
// 地址提供排序依据。BASE_URL_PROBE_INTERVAL_SECONDS <= 0 时关闭。
func StartBaseURLProbeTask() {
	interval := common.GetEnvOrDefault("BASE_URL_PROBE_INTERVAL_SECONDS", 60)
	if interval <= 0 {
		common.SysLog("base URL probe task disabled")
		return
	}
	go func() {
		probeBaseURLRoutes()
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			probeBaseURLRoutes()
		}
	}()
}

func probeBaseURLRoutes() {
	channels, err := model.GetChannelsForBaseURLProbe()
	if err != nil {
		common.SysError("list channels for base URL probe: " + err.Error())
		return
	}
	targets := make(map[int][]string)
	for _, channel := range channels {
		urls := channel.GetBaseURLs()
		if len(urls) <= 1 {
			continue
		}
		defaultBaseURLRouter.ensureRoute(channel.Id, urls)
		targets[channel.Id] = urls
	}
	if len(targets) == 0 {
		return
	}
	sem := make(chan struct{}, baseURLProbeWorkers)
	var wg sync.WaitGroup
	for channelID, urls := range targets {
		for _, rawURL := range urls {
			wg.Add(1)
			sem <- struct{}{}
			go func(channelID int, rawURL string) {
				defer wg.Done()
				defer func() { <-sem }()
				latency, ok := probeBaseURL(rawURL)
				ReportProbeLatency(channelID, rawURL, latency, ok)
			}(channelID, rawURL)
		}
	}
	wg.Wait()
}

// probeBaseURL 完成 TCP（https 时含 TLS）握手并返回耗时；不发送 HTTP 请求，
// 避免探测本身消耗上游配额。
func probeBaseURL(rawURL string) (time.Duration, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return 0, false
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	start := time.Now()
	dialer := &net.Dialer{Timeout: baseURLProbeTimeout}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	if parsed.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		ctx, cancel := context.WithTimeout(context.Background(), baseURLProbeTimeout)
		defer cancel()
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return 0, false
		}
	}
	return time.Since(start), true
}
