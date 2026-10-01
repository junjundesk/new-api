package common

import (
	"regexp"
	"strings"
)

// Email bodies are authored by admins as text in a form field, so the content
// often carries escape sequences typed literally ("第一行\n第二行") and real
// newlines from pressing Enter. Both must render as the author intended instead
// of showing up as the characters "\" and "n".
//
// Normalization happens in SendEmail so every mail path (verification, quota
// warnings, bonus notices, admin broadcasts) behaves the same.

// resolveEmailEscapes turns escape sequences into the characters they stand
// for, and normalizes real CR / CRLF byte pairs to a single LF. Unknown escapes
// are kept verbatim so nothing is silently dropped.
func resolveEmailEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]

		// 真实的 CRLF / CR 字节：统一成单个 \n。
		if c == '\r' {
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			b.WriteByte('\n')
			continue
		}
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}

		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
			i++
		case 'r':
			// "\r\n"（两个转义序列）与裸 "\r" 都表示一个换行。
			if i+3 < len(s) && s[i+2] == '\\' && s[i+3] == 'n' {
				i += 3
			} else {
				i++
			}
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
			i++
		case '\\':
			b.WriteByte('\\')
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// emailBlockTag lists elements that already lay out as their own block, so a
// newline next to one is formatting whitespace rather than a visible break.
const emailBlockTag = `(?:p|div|ul|ol|li|table|thead|tbody|tfoot|tr|td|th|h[1-6]|blockquote|section|article|header|footer|figure|figcaption|pre|address|dl|dt|dd|form|fieldset|nav|aside|main|details|summary)`

// emailBlockAfterNewline matches a line that opens or closes a block element.
var emailBlockAfterNewline = regexp.MustCompile(`(?is)^[ \t]*(?:</?` + emailBlockTag + `\b[^>]*>)`)

// emailBlockBeforeNewline matches a line ending in a block close or an explicit
// break, where adding another break would double the spacing.
var emailBlockBeforeNewline = regexp.MustCompile(`(?is)(?:</` + emailBlockTag + `>|<(?:br|hr)\b[^>]*>)[ \t]*$`)

// StripEmailHeaderBreaks removes CR/LF from a value written into an email
// header, preventing header injection and malformed messages.
func StripEmailHeaderBreaks(value string) string {
	if !strings.ContainsAny(value, "\r\n") {
		return value
	}
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(value)
}

// NormalizeEmailSubject prepares an admin-authored subject: escape sequences
// are resolved and any resulting line break collapses into a single space,
// because a header value cannot span lines.
func NormalizeEmailSubject(subject string) string {
	resolved := resolveEmailEscapes(subject)
	// Fields splits on any whitespace run, so this also collapses the repeated
	// spaces a removed newline can leave behind.
	return strings.Join(strings.Fields(resolved), " ")
}

// NormalizeEmailHTMLContent prepares an admin-authored HTML body: escape
// sequences are resolved, then newlines become explicit line breaks so text the
// author split across lines renders on separate lines instead of collapsing.
//
// Newlines adjacent to block-level tags are left alone: a body written as
// "<p>a</p>\n<p>b</p>" already renders on separate lines, and inserting a break
// there would add unwanted vertical space.
func NormalizeEmailHTMLContent(content string) string {
	resolved := resolveEmailEscapes(content)
	if !strings.Contains(resolved, "\n") {
		return resolved
	}

	lines := strings.Split(resolved, "\n")
	var b strings.Builder
	b.Grow(len(resolved) + len(lines)*6)
	for i, line := range lines {
		if i > 0 {
			previous := strings.TrimRight(lines[i-1], " \t\r")
			next := strings.TrimLeft(line, " \t")
			if emailBlockBeforeNewline.MatchString(previous) || emailBlockAfterNewline.MatchString(next) {
				b.WriteString("\n")
			} else {
				b.WriteString("<br/>\n")
			}
		}
		b.WriteString(line)
	}
	return b.String()
}
