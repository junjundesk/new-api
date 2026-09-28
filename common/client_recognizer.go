/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package common

import (
	"regexp"
	"strings"
)

// ClientInfo 描述从请求 User-Agent 中识别出的调用客户端。
// 以 JSON 存入消费日志 other 字段的 "client" 键，前端渲染为
// 可点击芯片 + 详情弹窗（参考站同款交互）。
type ClientInfo struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Variant    string `json:"variant,omitempty"`
	Version    string `json:"version,omitempty"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
}

const (
	clientSourceUserAgent = "User-Agent"
	clientConfIdentified  = "identified"
	clientConfGuessed     = "guessed"
	clientConfUnknown     = "unknown"
)

// 客户端类别（稳定枚举值，前端负责 i18n）
const (
	clientCategoryCoding   = "coding"   // 编程客户端
	clientCategoryChat     = "chat"     // 聊天客户端
	clientCategoryBrowser  = "browser"  // 浏览器 / 网页应用
	clientCategoryAgent    = "agent"    // 智能体 / 自动化
	clientCategoryMedia    = "media"    // 媒体生成
	clientCategoryOffice   = "office"   // 办公 / 效率
	clientCategoryTranslat = "translation" // 翻译
	clientCategoryGateway  = "gateway"  // 网关 / 代理
	clientCategoryLibrary  = "library"  // SDK / 库
	clientCategoryOther    = "other"    // 其他
)

// clientRule 按声明顺序匹配，先命中先生效。
type clientRule struct {
	pattern      *regexp.Regexp
	category     string
	variant      string
	versionGroup int // 1-based 捕获组下标，0 表示不提取版本
	// name 为展示名；为空时使用 firstCaptureGroup 作为名称（规范化首字母）
	name              string
	firstCaptureAsName bool
}

var clientRules = []clientRule{
	// ---- 编程客户端 ----
	{regexp.MustCompile("(?i)claude[ -]?code/([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Claude Code", false},
	{regexp.MustCompile("(?i)claude[ -]?code"), clientCategoryCoding, "cli", 0, "Claude Code", false},
	{regexp.MustCompile("(?i)claude[ -]?desktop/([0-9][^ )]*)"), clientCategoryCoding, "desktop", 1, "Claude Desktop", false},
	{regexp.MustCompile("(?i)claude[ -]?desktop"), clientCategoryCoding, "desktop", 0, "Claude Desktop", false},
	// Codex Desktop：显式 desktop 标记（如 "Codex Desktop/0.158.0 (...)" 或 "codex_desktop"）
	{regexp.MustCompile("(?i)codex[ _-]?desktop[/ ]([0-9][^ )]*)"), clientCategoryCoding, "desktop", 1, "Codex Desktop", false},
	{regexp.MustCompile("(?i)codex[ _-]?desktop"), clientCategoryCoding, "desktop", 0, "Codex Desktop", false},
	// Codex CLI：形如 "codex-cli/0.1.0" 或 "Codex/1.2 (cli)"
	{regexp.MustCompile("(?i)codex[ _-]?cli[/ ]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Codex CLI", false},
	{regexp.MustCompile("(?i)codex[ _-]?cli"), clientCategoryCoding, "cli", 0, "Codex CLI", false},
	{regexp.MustCompile("(?i)\bcodex[/ ]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Codex", false},
	{regexp.MustCompile("(?i)\bcodex\b"), clientCategoryCoding, "cli", 0, "Codex", false},
	{regexp.MustCompile("(?i)cursor/([0-9][^ )]*)"), clientCategoryCoding, "editor", 1, "Cursor", false},
	{regexp.MustCompile("(?i)cursor"), clientCategoryCoding, "editor", 0, "Cursor", false},
	{regexp.MustCompile("(?i)windsurf[ /-]?([0-9][^ )]*)"), clientCategoryCoding, "editor", 1, "Windsurf", false},
	{regexp.MustCompile("(?i)windsurf"), clientCategoryCoding, "editor", 0, "Windsurf", false},
	{regexp.MustCompile("(?i)trae[ /-]?([0-9][^ )]*)"), clientCategoryCoding, "editor", 1, "Trae", false},
	{regexp.MustCompile("(?i)trae"), clientCategoryCoding, "editor", 0, "Trae", false},
	{regexp.MustCompile("(?i)aider[ /]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Aider", false},
	{regexp.MustCompile("(?i)aider"), clientCategoryCoding, "cli", 0, "Aider", false},
	{regexp.MustCompile("(?i)cline[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "Cline", false},
	{regexp.MustCompile("(?i)cline"), clientCategoryCoding, "agent", 0, "Cline", false},
	{regexp.MustCompile("(?i)roo-?code[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "Roo Code", false},
	{regexp.MustCompile("(?i)roo-?code"), clientCategoryCoding, "agent", 0, "Roo Code", false},
	{regexp.MustCompile("(?i)kilo[ -]?code[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "Kilo Code", false},
	{regexp.MustCompile("(?i)kilo[ -]?code"), clientCategoryCoding, "agent", 0, "Kilo Code", false},
	{regexp.MustCompile("(?i)github[ -]?copilot[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "GitHub Copilot", false},
	{regexp.MustCompile("(?i)github[ -]?copilot"), clientCategoryCoding, "agent", 0, "GitHub Copilot", false},
	{regexp.MustCompile("(?i)open[ -]?interpreter[/ ]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Open Interpreter", false},
	{regexp.MustCompile("(?i)open[ -]?interpreter"), clientCategoryCoding, "cli", 0, "Open Interpreter", false},
	{regexp.MustCompile("(?i)zed[/ ]([0-9][^ )]*)"), clientCategoryCoding, "editor", 1, "Zed", false},
	{regexp.MustCompile("(?i)\bzed\b"), clientCategoryCoding, "editor", 0, "Zed", false},
	{regexp.MustCompile("(?i)continue[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "Continue", false},
	{regexp.MustCompile("(?i)continuedev"), clientCategoryCoding, "agent", 0, "Continue", false},
	{regexp.MustCompile("(?i)goose[/ ]([0-9][^ )]*)"), clientCategoryCoding, "agent", 1, "Goose", false},
	{regexp.MustCompile("(?i)\bgoose\b"), clientCategoryCoding, "agent", 0, "Goose", false},
	{regexp.MustCompile("(?i)opencode[/ ]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "OpenCode", false},
	{regexp.MustCompile("(?i)\bopencode\b"), clientCategoryCoding, "cli", 0, "OpenCode", false},
	{regexp.MustCompile("(?i)gemini[ -]?cli[/ ]([0-9][^ )]*)"), clientCategoryCoding, "cli", 1, "Gemini CLI", false},
	{regexp.MustCompile("(?i)gemini[ -]?cli"), clientCategoryCoding, "cli", 0, "Gemini CLI", false},
	{regexp.MustCompile("(?i)jetbrains|intellij|goland|pycharm|webstorm|clion|rubymine|rider|datagrip|dataspell|\bfleet\b"), clientCategoryCoding, "ide", 0, "JetBrains AI", false},
	{regexp.MustCompile("(?i)\bvscode\b|visual studio code"), clientCategoryCoding, "editor", 0, "VS Code", false},
	{regexp.MustCompile("(?i)voideditor|\bvoid\b"), clientCategoryCoding, "editor", 0, "Void", false},

	// ---- 聊天客户端 ----
	{regexp.MustCompile("(?i)chatbox[/ ]([0-9][^ )]*)"), clientCategoryChat, "app", 1, "Chatbox", false},
	{regexp.MustCompile("(?i)chatbox"), clientCategoryChat, "app", 0, "Chatbox", false},
	{regexp.MustCompile("(?i)cherry[ -]?studio[/ ]([0-9][^ )]*)"), clientCategoryChat, "app", 1, "Cherry Studio", false},
	{regexp.MustCompile("(?i)cherry[ -]?studio"), clientCategoryChat, "app", 0, "Cherry Studio", false},
	{regexp.MustCompile("(?i)nextchat[/ ]([0-9][^ )]*)"), clientCategoryChat, "web", 1, "NextChat", false},
	{regexp.MustCompile("(?i)nextchat"), clientCategoryChat, "web", 0, "NextChat", false},
	{regexp.MustCompile("(?i)lobe[ -]?chat[/ ]([0-9][^ )]*)"), clientCategoryChat, "web", 1, "LobeChat", false},
	{regexp.MustCompile("(?i)lobe[ -]?chat"), clientCategoryChat, "web", 0, "LobeChat", false},
	{regexp.MustCompile("(?i)openwebui|open-webui"), clientCategoryChat, "web", 0, "Open WebUI", false},
	{regexp.MustCompile("(?i)librechat"), clientCategoryChat, "web", 0, "LibreChat", false},

	{regexp.MustCompile("(?i)chatgpt[ -]?next"), clientCategoryChat, "web", 0, "ChatGPT Next Web", false},
	{regexp.MustCompile("(?i)bob[ /]([0-9][^ )]*)"), clientCategoryTranslat, "app", 1, "Bob", false},
	{regexp.MustCompile("(?i)openai[ -]?translator"), clientCategoryTranslat, "app", 0, "OpenAI Translator", false},
	{regexp.MustCompile("(?i)pot[ -]?app"), clientCategoryTranslat, "app", 0, "Pot", false},
	{regexp.MustCompile("(?i)immersivetranslate|immersive[ -]?translate"), clientCategoryTranslat, "extension", 0, "Immersive Translate", false},

	// ---- 智能体 / 自动化 ----
	{regexp.MustCompile("(?i)dify"), clientCategoryAgent, "platform", 0, "Dify", false},
	{regexp.MustCompile("(?i)coze"), clientCategoryAgent, "platform", 0, "Coze", false},
	{regexp.MustCompile("(?i)n8n"), clientCategoryAgent, "automation", 0, "n8n", false},
	{regexp.MustCompile("(?i)langchain|langgraph"), clientCategoryAgent, "framework", 0, "LangChain", false},
	{regexp.MustCompile("(?i)autogen"), clientCategoryAgent, "framework", 0, "AutoGen", false},
	{regexp.MustCompile("(?i)crewai"), clientCategoryAgent, "framework", 0, "CrewAI", false},
	{regexp.MustCompile("(?i)openai[ -]?agents"), clientCategoryAgent, "framework", 0, "OpenAI Agents", false},
	{regexp.MustCompile("(?i)sillytavern"), clientCategoryAgent, "roleplay", 0, "SillyTavern", false},

	// ---- 网关 / 代理（说明请求来自另一个网关）----
	{regexp.MustCompile("(?i)new-?api"), clientCategoryGateway, "gateway", 0, "NewAPI", false},
	{regexp.MustCompile("(?i)one-?api"), clientCategoryGateway, "gateway", 0, "OneAPI", false},
	{regexp.MustCompile("(?i)openrouter"), clientCategoryGateway, "gateway", 0, "OpenRouter", false},

	// ---- 库 / SDK ----
	{regexp.MustCompile("(?i)openai[ /-]?python[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "OpenAI Python SDK", false},
	{regexp.MustCompile("(?i)openai[ /-]?nodejs[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "OpenAI JS SDK", false},
	{regexp.MustCompile("(?i)openai-python"), clientCategoryLibrary, "sdk", 0, "OpenAI Python SDK", false},
	{regexp.MustCompile("(?i)openai-node"), clientCategoryLibrary, "sdk", 0, "OpenAI JS SDK", false},
	{regexp.MustCompile("(?i)anthropic[ /-]?python[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "Anthropic Python SDK", false},
	{regexp.MustCompile("(?i)anthropic[ /-]?typescript[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "Anthropic TS SDK", false},
	{regexp.MustCompile("(?i)go-openai|sashabaranov/go-openai"), clientCategoryLibrary, "sdk", 0, "go-openai", false},
	{regexp.MustCompile("(?i)GuzzleHttp"), clientCategoryLibrary, "sdk", 0, "Guzzle HTTP", false},
	{regexp.MustCompile("(?i)axios[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "Axios", false},
	{regexp.MustCompile("(?i)python-requests|python-urllib"), clientCategoryLibrary, "sdk", 0, "Python Requests", false},
	{regexp.MustCompile("(?i)python-httpx"), clientCategoryLibrary, "sdk", 0, "Python httpx", false},
	{regexp.MustCompile("(?i)python-aiohttp"), clientCategoryLibrary, "sdk", 0, "Python aiohttp", false},
	{regexp.MustCompile("(?i)node-fetch"), clientCategoryLibrary, "sdk", 0, "node-fetch", false},
	{regexp.MustCompile("(?i)undici"), clientCategoryLibrary, "sdk", 0, "Undici", false},
	{regexp.MustCompile("(?i)got[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "got", false},
	{regexp.MustCompile("(?i)Apache-HttpClient"), clientCategoryLibrary, "sdk", 0, "Apache HttpClient", false},
	{regexp.MustCompile("(?i)okhttp[/ ]([0-9][^ )]*)"), clientCategoryLibrary, "sdk", 1, "OkHttp", false},
	{regexp.MustCompile("(?i)dart:io|dart-io"), clientCategoryLibrary, "sdk", 0, "Dart IO", false},
	{regexp.MustCompile("(?i)rustls|reqwest"), clientCategoryLibrary, "sdk", 0, "Rust reqwest", false},

	// ---- 浏览器（最泛，靠后）----
	{regexp.MustCompile("(?i)edg[ea][/ ]([0-9.]+)"), clientCategoryBrowser, "browser", 1, "Edge", false},
	{regexp.MustCompile("(?i)opr[/ ]([0-9.]+)|opera"), clientCategoryBrowser, "browser", 1, "Opera", false},
	{regexp.MustCompile("(?i)firefox[/ ]([0-9.]+)"), clientCategoryBrowser, "browser", 1, "Firefox", false},
	{regexp.MustCompile("(?i)chrome[/ ]([0-9.]+)"), clientCategoryBrowser, "browser", 1, "Chrome", false},
	{regexp.MustCompile("(?i)safari[/ ]([0-9.]+)"), clientCategoryBrowser, "browser", 1, "Safari", false},
}

// recognizeClient 从 User-Agent 识别客户端信息，结果不缓存（正则均在启动期编译）。
func recognizeClient(userAgent string) *ClientInfo {
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return nil
	}
	for i := range clientRules {
		rule := &clientRules[i]
		m := rule.pattern.FindStringSubmatch(ua)
		if m == nil {
			continue
		}
		info := &ClientInfo{
			Category:   rule.category,
			Variant:    rule.variant,
			Source:     clientSourceUserAgent,
			Confidence: clientConfIdentified,
		}
		if rule.name != "" {
			info.Name = rule.name
		} else if len(m) > 1 {
			info.Name = m[1]
		}
		if rule.versionGroup > 0 && rule.versionGroup < len(m) && m[rule.versionGroup] != "" {
			info.Version = m[rule.versionGroup]
		}
		return info
	}
	// 兜底：取 UA 第一段作为名称，标记为 guessed
	if idx := strings.IndexAny(ua, " /("); idx > 0 {
		name := strings.TrimSpace(ua[:idx])
		if name != "" && len(name) <= 64 {
			return &ClientInfo{
				Name:       name,
				Category:   clientCategoryOther,
				Source:     clientSourceUserAgent,
				Confidence: clientConfGuessed,
			}
		}
	}
	return &ClientInfo{
		Name:       "unknown",
		Category:   clientCategoryOther,
		Source:     clientSourceUserAgent,
		Confidence: clientConfUnknown,
	}
}

// AttachClientInfoToOther 将客户端识别信息写入日志 other map：
// other["client"] = 结构化信息，other["user_agent"] = 原始 UA（截断）。
// UA 原文与 client 均为用户侧可见字段（非 admin_info）。
func AttachClientInfoToOther(other map[string]interface{}, userAgent string) {
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return
	}
	if other == nil {
		other = make(map[string]interface{})
	}
	if _, exists := other["user_agent"]; !exists {
		// 截断超长 UA，防止异常值撑爆日志行
		if len(ua) > 512 {
			ua = ua[:512]
		}
		other["user_agent"] = ua
	}
	if _, exists := other["client"]; !exists {
		if info := recognizeClient(ua); info != nil {
			other["client"] = info
		}
	}
}