package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEmailEscapes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "literal newline escape", input: `第一行\n第二行`, want: "第一行\n第二行"},
		{name: "literal crlf escape", input: `a\r\nb`, want: "a\nb"},
		{name: "bare cr escape", input: `a\rb`, want: "a\nb"},
		{name: "tab escape", input: `a\tb`, want: "a\tb"},
		{name: "escaped backslash", input: `a\\nb`, want: `a\nb`},
		{name: "unknown escape kept verbatim", input: `a\qb`, want: `a\qb`},
		{name: "trailing backslash kept", input: `abc\`, want: `abc\`},
		{name: "no escapes untouched", input: "普通文本", want: "普通文本"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveEmailEscapes(tc.input))
		})
	}
}

func TestNormalizeEmailSubjectCollapsesToSingleLine(t *testing.T) {
	assert.Equal(t, "第一行 第二行", NormalizeEmailSubject(`第一行\n第二行`))
	assert.Equal(t, "a b", NormalizeEmailSubject("a\n\n\nb"))
	assert.Equal(t, "notice", NormalizeEmailSubject("  notice  "))

	// A subject must never carry characters that would break the header.
	subject := NormalizeEmailSubject("real\r\nnewline")
	assert.NotContains(t, subject, "\n")
	assert.NotContains(t, subject, "\r")
	assert.Equal(t, "real newline", subject)
}

func TestNormalizeEmailHTMLContentTurnsNewlinesIntoBreaks(t *testing.T) {
	assert.Equal(t, "第一行<br/>\n第二行", NormalizeEmailHTMLContent(`第一行\n第二行`))
	assert.Equal(t, "第一行<br/>\n第二行", NormalizeEmailHTMLContent("第一行\n第二行"))
}

// TestNormalizeEmailHTMLContentKeepsStructuredHTML guards the built-in default
// template: paragraph tags already put each block on its own line, so the
// newlines between them must not gain extra breaks.
func TestNormalizeEmailHTMLContentKeepsStructuredHTML(t *testing.T) {
	structured := "<p>a</p>\n<p>b</p>\n<p>c</p>"
	assert.Equal(t, structured, NormalizeEmailHTMLContent(structured))

	// The shipped bonus template must survive normalization unchanged, with
	// escaped newlines treated exactly like real ones.
	const template = "<p>您好 $display_name，</p>\n<p>您已收到一笔赠金：<b>$amount</b>。</p>\n<p>有效期：$expire_text</p>\n$remark_block"
	assert.Equal(t, template, NormalizeEmailHTMLContent(template))

	escaped := strings.ReplaceAll(template, "\n", `\n`)
	assert.Equal(t, template, NormalizeEmailHTMLContent(escaped))
}

func TestNormalizeEmailHTMLContentDoesNotDoubleExplicitBreaks(t *testing.T) {
	content := "a<br/>\nb"
	assert.Equal(t, content, NormalizeEmailHTMLContent(content))
}

func TestNormalizeEmailHTMLContentHandlesMixedAuthoredText(t *testing.T) {
	// Text the author split with both real newlines and escapes.
	got := NormalizeEmailHTMLContent("line1\r\nline2\\nline3")
	assert.Equal(t, "line1<br/>\nline2<br/>\nline3", got)
	assert.False(t, strings.Contains(got, `\n`), "no escape sequence may survive")
}

func TestStripEmailHeaderBreaks(t *testing.T) {
	assert.Equal(t, "a b", StripEmailHeaderBreaks("a\r\nb"))
	assert.Equal(t, "a b", StripEmailHeaderBreaks("a\nb"))
	assert.Equal(t, "a b", StripEmailHeaderBreaks("a\rb"))
	assert.Equal(t, "plain@example.com", StripEmailHeaderBreaks("plain@example.com"))
}

// TestNormalizeEmailSubjectBlocksHeaderInjection covers the security intent:
// a subject carrying CRLF must not be able to start a new header.
func TestNormalizeEmailSubjectBlocksHeaderInjection(t *testing.T) {
	injected := NormalizeEmailSubject("hello\r\nBcc: attacker@example.com")
	require.NotContains(t, injected, "\r")
	require.NotContains(t, injected, "\n")
	assert.Equal(t, "hello Bcc: attacker@example.com", injected)

	// The same holds when the CRLF was typed as an escape sequence.
	escaped := NormalizeEmailSubject(`hello\r\nBcc: attacker@example.com`)
	assert.NotContains(t, escaped, "\n")
}
