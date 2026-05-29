package database

import "strings"

// rewriteSQLite converts a pgx-style query (numbered `$N` placeholders, ILIKE,
// now()) into a SQLite-compatible query (`?` placeholders, LIKE,
// CURRENT_TIMESTAMP) and rebuilds the argument slice to match.
//
// Placeholders are emitted positionally: each `$N` occurrence becomes a single
// `?` and the corresponding args[N-1] value is appended to the new slice in
// order. This correctly handles a placeholder that is reused within a query
// (e.g. the keyword filter `title ILIKE $1 OR description ILIKE $1`), which
// SQLite's positional `?` binding requires to be duplicated.
//
// `$N` tokens inside single-quoted string literals are left untouched.
func rewriteSQLite(query string, args []any) (string, []any) {
	query = replaceTokens(query)

	var b strings.Builder
	b.Grow(len(query))
	newArgs := make([]any, 0, len(args))

	inString := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == '\'' {
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if c == '$' && !inString && i+1 < len(query) && isDigit(query[i+1]) {
			j := i + 1
			for j < len(query) && isDigit(query[j]) {
				j++
			}
			n := atoi(query[i+1 : j])
			b.WriteByte('?')
			if n >= 1 && n <= len(args) {
				newArgs = append(newArgs, args[n-1])
			} else {
				newArgs = append(newArgs, nil)
			}
			i = j - 1
			continue
		}
		b.WriteByte(c)
	}

	return b.String(), newArgs
}

// replaceTokens swaps PostgreSQL-only keywords/functions for SQLite equivalents
// on word boundaries, outside of string literals. SQLite's LIKE is
// case-insensitive for ASCII, matching ILIKE's intent for keyword search.
func replaceTokens(query string) string {
	var b strings.Builder
	b.Grow(len(query))
	inString := false
	for i := 0; i < len(query); {
		c := query[i]
		if c == '\'' {
			inString = !inString
			b.WriteByte(c)
			i++
			continue
		}
		if !inString {
			if matchWord(query, i, "ILIKE") {
				b.WriteString("LIKE")
				i += len("ILIKE")
				continue
			}
			if matchWord(query, i, "now()") {
				b.WriteString("CURRENT_TIMESTAMP")
				i += len("now()")
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// matchWord reports whether query[i:] begins with word (case-insensitive) at a
// word boundary. For "now()" the trailing "()" already bounds the match; for
// "ILIKE" we require a non-identifier char on both sides.
func matchWord(query string, i int, word string) bool {
	if i+len(word) > len(query) {
		return false
	}
	if !strings.EqualFold(query[i:i+len(word)], word) {
		return false
	}
	// Left boundary.
	if i > 0 && isIdentChar(query[i-1]) {
		return false
	}
	// Right boundary: only enforce when the word ends in an identifier char.
	last := word[len(word)-1]
	if isIdentChar(last) {
		if i+len(word) < len(query) && isIdentChar(query[i+len(word)]) {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isIdentChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || isDigit(c)
}

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
