/*
Package glob implements glob pattern match.
This is implemented according to IEEE Std 1003.1-202x.

This package does not cover the filename expansion pattern.
If you want to use filename expansion pattern, use filepath.Glob.
*/
package glob

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type matchPattern int

const (
	equalPatten matchPattern = iota
	globPattern
	backwardPattern
	forwardPattern
	forwardBackwardPattern
)

type kind int

const (
	runeKind kind = iota + 1
	astariskKind
	questionKind
	regexKind
)

type matcher interface {
	match(r rune) bool
	kind() kind
}

type (
	runeMatcher     struct{ rune }
	astariskMatcher struct{}
	questionMatcher struct{}
	regexMatcher    struct{ *regexp.Regexp }
)

func (m runeMatcher) match(r rune) bool {
	return m.rune == r
}

func (m runeMatcher) kind() kind {
	return runeKind
}

func (astariskMatcher) match(r rune) bool {
	return true
}

func (astariskMatcher) kind() kind {
	return astariskKind
}

func (questionMatcher) match(r rune) bool {
	return true
}

func (questionMatcher) kind() kind {
	return questionKind
}

func (m regexMatcher) match(r rune) bool {
	return m.MatchString(string(r))
}

func (m regexMatcher) kind() kind {
	return regexKind
}

// Glob has compiled pattern.
type Glob struct {
	pattern []matcher
	str     string

	ddlPool sync.Pool

	runeNums int

	matchPattern matchPattern
	matchIndex   int
}

const defaultBuf = 1 << 10

// Compile compile Glob from given pattern.
func Compile(pattern string) (*Glob, error) {
	var (
		matchp     matchPattern
		matchIndex int
		lastIndex  int
		escape     bool
	)
loopend:
	for i, r := range pattern {
		switch {
		case !escape && r == '\\':
			escape = true
			continue

		case r == '?', r == '[':
			matchp = globPattern
			break loopend

		// backward
		case i == 0 && r == '*':
			matchp = backwardPattern
			matchIndex = len(pattern) - 1

		case matchp == backwardPattern && r == '*':
			if pattern[i-1] == '*' {
				matchIndex--
				continue
			}
			matchIndex = len(pattern) - matchIndex
			lastIndex = i
			matchp = forwardBackwardPattern

		// forwardBackward
		case matchp == forwardBackwardPattern && r != '*':
			matchp = globPattern
			break loopend

		// forward
		case matchp == equalPatten && r == '*':
			matchp = forwardPattern
			matchIndex = i

		case matchp == forwardPattern && r != '*':
			matchp = globPattern
			break loopend
		}
		escape = false
	}
	if matchp == globPattern {
		matchers := make([]matcher, 0, len(pattern))
		var (
			escape bool
			runes  = []rune(pattern)
		)
		for i := 0; i < len(runes); i++ {
			switch {
			case !escape && runes[i] == '\\':
				escape = true
				continue
			case !escape && runes[i] == '*':
				matchers = append(matchers, astariskMatcher{})
			case !escape && runes[i] == '?':
				matchers = append(matchers, questionMatcher{})
			case !escape && runes[i] == '[':
				end := indexCloseSquare(pattern[i:])
				if end == -1 {
					return nil, fmt.Errorf("there is no ']' corresponding to '['")
				}
				regex, err := regexp.Compile(pattern[i : i+end+1])
				if err != nil {
					return nil, err
				}
				matchers = append(matchers, regexMatcher{regex})
				i += end
			default:
				matchers = append(matchers, runeMatcher{runes[i]})
			}
			escape = false
		}
		runeNums := len(matchers)
		return &Glob{
			pattern:      matchers,
			str:          pattern,
			matchPattern: globPattern,
			ddlPool: sync.Pool{
				New: func() interface{} {
					ddl := make([][]bool, 0, runeNums)
					return &ddl
				},
			},
			runeNums: runeNums,
		}, nil
	}
	if matchp == forwardBackwardPattern {
		return &Glob{
			str:          pattern[matchIndex:lastIndex],
			matchPattern: matchp,
		}, nil
	}
	return &Glob{
		str:          pattern,
		matchPattern: matchp,
		matchIndex:   matchIndex,
	}, nil
}

func indexCloseSquare(str string) int {
	var (
		escape     bool
		match      bool
		matchIndex int
	)
	for i, r := range str {
		if match {
			if r == ']' {
				return i
			}
			return matchIndex
		}
		if !escape && r == '\\' {
			escape = true
			continue
		}
		if !escape && r == ']' {
			match = true
			matchIndex = i
		}
		escape = false
	}
	return -1
}

// Match returns whether the given string matches glob.
func (g *Glob) Match(s string) bool {
	switch g.matchPattern {
	case equalPatten:
		return g.str == s
	case forwardPattern:
		return len(s) >= g.matchIndex && s[:g.matchIndex] == g.str[:g.matchIndex]
	case backwardPattern:
		return len(s) >= g.matchIndex && s[len(s)-g.matchIndex:] == g.str[len(g.str)-g.matchIndex:]
	case forwardBackwardPattern:
		return strings.Contains(s, g.str)
	}

	str := []rune(s)
	pool := g.ddlPool.Get().(*[][]bool)
	dp := *pool
	dp = dp[:cap(dp)]

	var (
		firstMatch int
		dpIndex    int
	)
	for pIndex, r := range g.pattern {
		if cap(dp[dpIndex]) > 0 {
			dp[dpIndex] = dp[dpIndex][:0]
		} else if dpIndex > 2 {
			dp[dpIndex] = dp[dpIndex-2][:0]
		} else {
			dp[dpIndex] = make([]bool, 0, defaultBuf)
		}
		var match bool
		switch g.pattern[dpIndex].kind() {
		case astariskKind:
			if dpIndex == len(g.pattern)-1 {
				*pool = dp[:0]
				g.ddlPool.Put(pool)
				return true
			}
			for j := 0; j < len(s)-firstMatch; j++ {
				dp[dpIndex] = append(dp[dpIndex], true)
			}
		case questionKind, regexKind:
			for arrayj, strj := 0, firstMatch; strj < len(str); arrayj, strj = arrayj+1, strj+1 {
				m := (dpIndex == 0 && arrayj == 0) || (dpIndex != 0 && arrayj != 0 && dp[dpIndex-1][arrayj-1])
				m = m && r.match(str[strj])
				if !match {
					if !m {
						continue
					}
					match = true
					firstMatch = strj
				}
				dp[dpIndex] = append(dp[dpIndex], m)
			}
		case runeKind:
			if dpIndex == 0 {
				if !r.match(str[0]) {
					*pool = dp[:0]
					g.ddlPool.Put(pool)
					return false
				}
			}
			if dpIndex != 0 && g.pattern[pIndex-1].kind() == astariskKind {
				for arrayj, strj := 0, firstMatch; strj < len(str); arrayj, strj = arrayj+1, strj+1 {
					m := (dpIndex == 0 || dp[dpIndex-1][arrayj]) && r.match(str[strj])
					if !match {
						if !m {
							continue
						}
						match = true
						firstMatch = strj
					}
					dp[dpIndex] = append(dp[dpIndex], m)
				}
			} else {
				for arrayj, strj := 0, firstMatch; strj < len(str); arrayj, strj = arrayj+1, strj+1 {
					var m bool
					if dpIndex == 0 {
						m = arrayj == 0 && r.match(str[strj])
					} else {
						m = arrayj != 0 && dp[dpIndex-1][arrayj-1] && r.match(str[strj])
					}
					if !match {
						if !m {
							continue
						}
						match = true
						firstMatch = strj
					}
					dp[dpIndex] = append(dp[dpIndex], m)
				}
			}
		}
		if len(dp[dpIndex]) == 0 {
			*pool = dp[:0]
			g.ddlPool.Put(pool)
			return false
		}
		dpIndex++
	}
	*pool = dp[:0]
	g.ddlPool.Put(pool)
	return len(dp[g.runeNums-1])+firstMatch == len(str) && dp[g.runeNums-1][len(dp[g.runeNums-1])-1]
}

func (g *Glob) String() string {
	return g.str
}
