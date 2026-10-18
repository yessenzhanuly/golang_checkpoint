package password

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Rule is one requirement a strong password should satisfy.
// New requirements are added by implementing this interface; the Analyzer
// does not need to change.
type Rule interface {
	// Name describes the requirement. It is reported when the rule is not met.
	Name() string
	// Points is the weight of the rule in the final score.
	Points() int
	// Blocking rules cap the score when violated, no matter how many other
	// rules pass (e.g. a very common password is weak even if it is long).
	Blocking() bool
	// Passes reports whether the password satisfies the rule.
	Passes(password string) bool
}

// funcRule is a Rule backed by a check function. The built-in rules below
// are all instances of it, which keeps them short.
type funcRule struct {
	name     string
	points   int
	blocking bool
	check    func(password string) bool
}

func (r funcRule) Name() string                { return r.name }
func (r funcRule) Points() int                 { return r.points }
func (r funcRule) Blocking() bool              { return r.blocking }
func (r funcRule) Passes(password string) bool { return r.check(password) }

// MinLength requires at least n characters (runes, not bytes).
func MinLength(n, points int) Rule {
	return funcRule{
		name:   fmt.Sprintf("at least %d characters", n),
		points: points,
		check:  func(p string) bool { return utf8.RuneCountInString(p) >= n },
	}
}

// RequireLower requires a lowercase letter.
func RequireLower(points int) Rule {
	return funcRule{"a lowercase letter", points, false, func(p string) bool { return anyRune(p, unicode.IsLower) }}
}

// RequireUpper requires an uppercase letter.
func RequireUpper(points int) Rule {
	return funcRule{"an uppercase letter", points, false, func(p string) bool { return anyRune(p, unicode.IsUpper) }}
}

// RequireDigit requires a digit.
func RequireDigit(points int) Rule {
	return funcRule{"a digit", points, false, func(p string) bool { return anyRune(p, unicode.IsDigit) }}
}

// RequireSymbol requires a punctuation mark or a symbol.
func RequireSymbol(points int) Rule {
	return funcRule{"a symbol or punctuation mark", points, false, func(p string) bool {
		return anyRune(p, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })
	}}
}

// NoTripleRepeat forbids the same character three or more times in a row.
func NoTripleRepeat(points int) Rule {
	return funcRule{"no character repeated 3 or more times in a row", points, false, func(p string) bool {
		var prev rune
		run := 0
		for _, r := range p {
			if run > 0 && r == prev {
				run++
			} else {
				prev, run = r, 1
			}
			if run >= 3 {
				return false
			}
		}
		return true
	}}
}

// commonPasswords is a tiny sample of the most frequently used passwords.
var commonPasswords = map[string]struct{}{
	"password": {}, "password1": {}, "password123": {}, "123456": {},
	"123456789": {}, "12345678": {}, "111111": {}, "qwerty": {},
	"qwerty123": {}, "abc123": {}, "letmein": {}, "welcome": {},
	"admin": {}, "iloveyou": {},
}

// NotCommon rejects well-known passwords (case-insensitive). It is a blocking
// rule: a common password can never be rated better than weak.
func NotCommon(points int) Rule {
	return funcRule{"not a commonly used password", points, true, func(p string) bool {
		_, found := commonPasswords[strings.ToLower(p)]
		return !found
	}}
}

// DefaultRules returns the standard rule set; the weights add up to 100.
func DefaultRules() []Rule {
	return []Rule{
		MinLength(8, 20),
		MinLength(12, 15),
		RequireLower(10),
		RequireUpper(10),
		RequireDigit(10),
		RequireSymbol(15),
		NoTripleRepeat(10),
		NotCommon(10),
	}
}

// anyRune reports whether at least one rune of s satisfies f.
func anyRune(s string, f func(rune) bool) bool {
	for _, r := range s {
		if f(r) {
			return true
		}
	}
	return false
}
