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

import "strings"

// 性能页「Coding cache / Coding 输入成本」的客户端白名单口径，来自参考站
// 弹层原文：Codex、Claude Code、Pi、OpenCode、OMP、ZCode、DeepSeek Harness
// (DSH)、Open Design。该口径只用于判定「这次请求的输入侧是否计入 Coding
// 样本」，与 common.ClientInfo 中更宽的 clientCategoryCoding（还包含 Cursor、
// VS Code 等编辑器）刻意区分——白名单外的编程客户端不计入 Coding 样本。
//
// 判定只依赖请求 User-Agent，匹配方式为「小写化后按 token 边界匹配」：
//   - 带空白/连字符分隔的产品名按子串匹配（Claude Code、DeepSeek Harness）；
//   - 单词型产品名要求出现在 UA 开头或紧跟在分隔符之后，避免
//     "open-design" 这类子串误伤无关客户端。
const (
	codingClientTokenSeparators = " \t/_;:,()[]{}<>|\\\"'+="
)

// IsCodingClientUserAgent 报告 User-Agent 是否属于性能页认定的 Coding 客户端。
func IsCodingClientUserAgent(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return false
	}
	// 分隔符形态：产品名内部含空格或连字符。
	for _, name := range []string{"claude code", "claude-code", "deepseek harness", "open design", "open-design"} {
		if strings.Contains(ua, name) {
			return true
		}
	}
	// 单词形态：必须在词边界上出现，避免子串误判。
	for _, name := range []string{"codex", "opencode", "omp", "zcode", "dsh", "pi"} {
		if hasClientToken(ua, name) {
			return true
		}
	}
	return false
}

// hasClientToken 报告 name 是否作为完整词元出现在 ua 中（小写、已 trim）。
func hasClientToken(ua string, name string) bool {
	from := 0
	for {
		idx := strings.Index(ua[from:], name)
		if idx < 0 {
			return false
		}
		start := from + idx
		end := start + len(name)
		leftOK := start == 0 || isClientTokenSeparator(ua[start-1])
		rightOK := end == len(ua) || isClientTokenSeparator(ua[end])
		if leftOK && rightOK {
			return true
		}
		from = start + 1
		if from >= len(ua) {
			return false
		}
	}
}

func isClientTokenSeparator(b byte) bool {
	return strings.IndexByte(codingClientTokenSeparators, b) >= 0
}
