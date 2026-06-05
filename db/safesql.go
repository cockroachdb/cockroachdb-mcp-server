package db

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/lexbase"
	"github.com/cockroachdb/errors"
)

// SQLStringer renders itself as a SQL fragment (identifier, literal, or raw SQL).
type SQLStringer interface {
	SQLString() string
}

// Identifier renders as a double-quoted SQL identifier via lexbase.
type Identifier string

// SQLString implements SQLStringer.
func (i Identifier) SQLString() string {
	return lexbase.EscapeSQLIdent(string(i))
}

// QualifiedIdentifier renders as dot-joined quoted identifiers ("db"."schema"."tbl").
type QualifiedIdentifier []string

// SQLString implements SQLStringer.
func (q QualifiedIdentifier) SQLString() string {
	parts := make([]string, len(q))
	for i, p := range q {
		parts[i] = lexbase.EscapeSQLIdent(p)
	}
	return strings.Join(parts, ".")
}

// SQL is a raw SQL fragment inlined verbatim. The caller is responsible for
// the safety of its contents.
type SQL string

// SQLString implements SQLStringer.
func (s SQL) SQLString() string { return string(s) }

// SafeFormat substitutes %N placeholders with quoted args: SQLStringer values
// render themselves, strings become SQL literals, int64 renders as decimal.
// Each placeholder must be preceded by punctuation, whitespace, or '=' so that
// `col%1` cannot silently become `col'value'`.
func SafeFormat(format string, args ...any) (string, error) {
	var b strings.Builder
	for {
		chunk, idx, rest, err := splitNext(format)
		if err != nil {
			return "", err
		}
		b.WriteString(chunk)
		if idx == 0 {
			return b.String(), nil
		}
		if idx > len(args) {
			return "", errors.Newf("safesql: placeholder %%%d out of range (have %d args)", idx, len(args))
		}
		s, err := render(args[idx-1])
		if err != nil {
			return "", err
		}
		b.WriteString(s)
		format = rest
	}
}

// splitNext finds the next %N placeholder in format. Returns the literal chunk
// before the placeholder, the parsed index, and the format remaining after it.
// If no placeholder is found, returns (format, 0, "", nil).
func splitNext(format string) (chunk string, idx int, rest string, err error) {
	j := strings.IndexByte(format, '%')
	if j < 0 {
		return format, 0, "", nil
	}
	if j > 0 {
		r := rune(format[j-1])
		if r != '=' && !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			return "", 0, "", errors.Newf("safesql: placeholder must follow punctuation or whitespace, got %q", r)
		}
	}
	p := format[j+1:]
	if len(p) == 0 || p[0] < '1' || p[0] > '9' {
		return "", 0, "", errors.Newf("safesql: invalid placeholder %q", format[j:])
	}
	n := 0
	for n < len(p) && p[n] >= '0' && p[n] <= '9' {
		if idx >= 1e6 {
			return "", 0, "", errors.Newf("safesql: placeholder index too large in %q", format[j:])
		}
		idx = idx*10 + int(p[n]-'0')
		n++
	}
	return format[:j], idx, format[j+1+n:], nil
}

func render(arg any) (string, error) {
	if s, ok := arg.(SQLStringer); ok {
		return s.SQLString(), nil
	}
	switch v := arg.(type) {
	case string:
		return lexbase.EscapeSQLString(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	default:
		return "", errors.Newf("safesql: unsupported type %T", arg)
	}
}
