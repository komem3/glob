/*
Package glob implements glob pattern match.
This is implemented according to IEEE Std 1003.1-2017.

This package does not cover the filename expansion pattern.
If you want to use filename expansion pattern, use filepath.Glob.

special chars:
	'?' A <question-mark> is a pattern that shall match any character.
	'*' An <asterisk> is a pattern that shall match multiple characters, as described in Patterns Matching Multiple Characters.
	'[' If an open bracket introduces a bracket expression as in RE Bracket Expression.
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
	reverseGlobPattern
	onePassPattern
	backwardPattern
	forwardPattern
	partialPattern
)

type kind int

const (
	startKind kind = iota + 1
	matchedKind
	runeKind
	asteriskKind
	questionKind
	regexKind
)

type matchFunc func(r rune) bool

type state struct {
	match  matchFunc
	kind   kind
	next   *state
	before *state
}

type dfaState struct {
	list          []*state
	asteriskNexts []*state
	next          map[rune]*dfaState
}

func alwaysTrue(_ rune) bool {
	return true
}

func alwaysFalse(_ rune) bool {
	return false
}

func regexMatchFunc(regex *regexp.Regexp) matchFunc {
	return func(r rune) bool {
		return regex.MatchString(string(r))
	}
}

func runeMatchFunc(target rune) matchFunc {
	return func(r rune) bool {
		return target == r
	}
}

// Glob has compiled pattern.
type Glob struct {
	str          string
	matchPattern matchPattern
	matchIndex   int

	dfaPool      *sync.Pool
	onePassState *state
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
func MustCompile(pattern string) *Glob {
	glob, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return glob
}

// Compile compile Glob from given pattern.
func Compile(pattern string) (*Glob, error) {
	var (
		matchp     = equalPatten
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

		case !escape && r == '?', r == '[':
			matchp = globPattern
			break loopend

		// backward
		case !escape && i == 0 && r == '*':
			matchp = backwardPattern
			matchIndex = len(pattern) - 1

		case !escape && matchp == backwardPattern && r == '*':
			if pattern[i-1] == '*' {
				matchIndex--
				continue
			}
			matchIndex = len(pattern) - matchIndex
			lastIndex = i
			matchp = partialPattern

		// partial
		case matchp == partialPattern && r != '*':
			matchp = globPattern
			break loopend

		// forward
		case !escape && matchp == equalPatten && r == '*':
			matchp = forwardPattern
			matchIndex = i

		case matchp == forwardPattern && r != '*':
			matchp = globPattern
			break loopend
		}
		escape = false
	}
	if matchp == globPattern {
		var (
			escape       bool
			matchPattern = onePassPattern
			patternState = &state{kind: startKind}
			startState   = patternState
			runes        = []rune(pattern)
		)
		for i := 0; i < len(runes); i++ {
			switch {
			case !escape && runes[i] == '\\':
				escape = true
				continue
			case !escape && runes[i] == '*':
				// convert multi asterisk to one. ** -> *
				if patternState.kind == asteriskKind {
					continue
				}
				patternState.next = &state{match: alwaysTrue, kind: asteriskKind, before: patternState}
				patternState = patternState.next
				matchPattern = globPattern
			case !escape && runes[i] == '?':
				patternState.next = &state{match: alwaysTrue, kind: questionKind, before: patternState}
				patternState = patternState.next
			case !escape && runes[i] == '[':
				end := indexCloseSquare(pattern[i:])
				if end == -1 {
					return nil, fmt.Errorf("there is no ']' corresponding to '['")
				}
				regex, err := regexp.Compile(pattern[i : i+end+1])
				if err != nil {
					return nil, err
				}
				patternState.next = &state{match: regexMatchFunc(regex), kind: regexKind, before: patternState}
				patternState = patternState.next
				i += end
			default:
				patternState.next = &state{match: runeMatchFunc(runes[i]), kind: runeKind, before: patternState}
				patternState = patternState.next
			}
			escape = false
		}
		patternState.next = &state{match: alwaysFalse, kind: matchedKind, before: patternState}

		// reverse state chain
		if len(runes) >= 2 && runes[0] == '*' && runes[len(runes)-1] != '*' {
			matchPattern = reverseGlobPattern
			startState.next = patternState
			for patternState.before.kind != startKind {
				patternState.next, patternState = patternState.before, patternState.before
			}
			patternState.next = &state{match: alwaysFalse, kind: matchedKind, before: patternState}
		}

		return &Glob{
			str:          pattern,
			matchPattern: matchPattern,
			onePassState: startState.next,
			dfaPool: &sync.Pool{New: func() interface{} {
				dfa := &dfaState{list: []*state{startState.next}, next: make(map[rune]*dfaState)}
				if startState.next.kind == asteriskKind {
					dfa.list = append(dfa.list, startState.next.next)
				}
				return dfa
			}},
		}, nil
	}
	if matchp == partialPattern {
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

func (g *Glob) onePassMatch(s string) bool {
	state := g.onePassState
	for _, r := range s {
		if !state.match(r) {
			return false
		}
		state = state.next
	}
	return state.kind == matchedKind
}

func (g *Glob) dfaMatch(s string) bool {
	dfa := g.dfaPool.Get().(*dfaState)
	startPtr := dfa

	for _, r := range s {
		next, ok := dfa.next[r]
		if ok {
			dfa = next
			continue
		}
		if len(dfa.list) == 0 && len(dfa.asteriskNexts) == 0 {
			return false
		}
		var (
			nlist         []*state
			asteriskNexts = dfa.asteriskNexts
		)
		for _, state := range dfa.list {
			if state.kind == asteriskKind {
				// The last asterisk matches all characters, so it's a match.
				if state.next.kind == matchedKind {
					g.dfaPool.Put(startPtr)
					return true
				}
				asteriskNexts = append(asteriskNexts, state.next)
				continue
			}
			if state.match(r) {
				nlist = append(nlist, state.next)
			}
		}
		for _, state := range dfa.asteriskNexts {
			if state.match(r) {
				nlist = append(nlist, state.next)
			}
		}
		next = &dfaState{list: nlist, asteriskNexts: asteriskNexts, next: make(map[rune]*dfaState)}
		if len(nlist) == 0 {
			next.next[r] = next
		}
		dfa.next[r] = next
		dfa = next
	}

	for _, st := range dfa.list {
		if st.kind == asteriskKind && st.next.kind == matchedKind {
			g.dfaPool.Put(startPtr)
			return true
		}
		if st.kind == matchedKind {
			g.dfaPool.Put(startPtr)
			return true
		}
	}
	g.dfaPool.Put(startPtr)
	return false
}

func (g *Glob) reverseMatch(s string) bool {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return g.dfaMatch(string(runes))
}

// Match returns whether the given string matches compiled glob.
func (g *Glob) Match(s string) bool {
	switch g.matchPattern {
	case equalPatten:
		return g.str == s
	case forwardPattern:
		return len(s) >= g.matchIndex && s[:g.matchIndex] == g.str[:g.matchIndex]
	case backwardPattern:
		return len(s) >= g.matchIndex && s[len(s)-g.matchIndex:] == g.str[len(g.str)-g.matchIndex:]
	case partialPattern:
		return strings.Contains(s, g.str)
	case onePassPattern:
		return g.onePassMatch(s)
	case globPattern:
		return g.dfaMatch(s)
	case reverseGlobPattern:
		return g.reverseMatch(s)
	}
	panic("unexpected pattern")
}

// String implements fmt.Stringer.
func (g *Glob) String() string {
	return g.str
}
