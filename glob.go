/*
Package glob implements glob pattern match.
This is implemented according to IEEE Std 1003.1-2017.

This package does not cover the filename expansion pattern.
If you want to use filename expansion pattern, use filepath.Glob.

special chars:
	'?' A <question-mark> is a pattern that shall match any character.
	'*' An <asterisk> is a pattern that shall match multiple characters, as described in Patterns Matching Multiple Characters.
	'[' If an open bracket introduces a bracket expression as in RE Bracket Expression. See regexp/syntax.
*/
package glob

import (
	"fmt"
	"regexp"
	"sync"
	"unicode/utf8"
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
	match matchFunc
	kind  kind
	next  *state
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

func ascStep(s string) (rune, string) {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return utf8.RuneError, ""
	}
	return r, s[size:]
}

func descStep(s string) (rune, string) {
	r, size := utf8.DecodeLastRuneInString(s)
	if r == utf8.RuneError {
		return utf8.RuneError, ""
	}
	return r, s[:len(s)-size]
}

// Glob has compiled pattern.
type Glob struct {
	str string

	dfaPool      *sync.Pool
	onePassState *state
	step         func(s string) (r rune, remain string)
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
func MustCompile(pattern string) *Glob {
	glob, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return glob
}

// Compile compiles Glob from given pattern.
func Compile(pattern string) (*Glob, error) {
	runes := []rune(pattern)
	// if exists last asterisk and not exist first asterisk, reverse pattern.
	reverse := len(runes) >= 2 && runes[0] == '*' && runes[len(runes)-1] != '*'
	if reverse {
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
	}

	var (
		escape        bool
		patternState  = &state{kind: startKind}
		startState    = patternState
		firstAsterisk bool
		useDFA        bool
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
			patternState.next = &state{kind: asteriskKind}
			patternState = patternState.next
			if i == 0 {
				firstAsterisk = true
				continue
			}
			if firstAsterisk || i != len(runes)-1 {
				useDFA = true
			}
		case !escape && runes[i] == '?':
			patternState.next = &state{match: alwaysTrue, kind: questionKind}
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
			patternState.next = &state{match: regexMatchFunc(regex), kind: regexKind}
			patternState = patternState.next
			i += end
		default:
			patternState.next = &state{match: runeMatchFunc(runes[i]), kind: runeKind}
			patternState = patternState.next
		}
		escape = false
	}
	patternState.next = &state{match: alwaysFalse, kind: matchedKind}

	glob := &Glob{
		str: pattern,
	}
	if reverse {
		glob.step = descStep
	} else {
		glob.step = ascStep
	}

	if useDFA {
		glob.dfaPool = &sync.Pool{New: func() interface{} {
			dfa := &dfaState{next: make(map[rune]*dfaState)}
			if startState.next.kind == asteriskKind {
				dfa.asteriskNexts = append(dfa.asteriskNexts, startState.next.next)
			} else {
				dfa.list = append(dfa.list, startState.next)
			}
			return dfa
		}}
	} else {
		glob.onePassState = startState.next
	}
	return glob, nil
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
	for r, s := g.step(s); r != utf8.RuneError; r, s = g.step(s) {
		if state.kind == asteriskKind {
			return true
		}
		if !state.match(r) {
			return false
		}
		state = state.next
	}
	return state.kind == asteriskKind || state.kind == matchedKind
}

func (g *Glob) dfaMatch(s string) bool {
	dfa := g.dfaPool.Get().(*dfaState)
	startPtr := dfa

	for r, s := g.step(s); r != utf8.RuneError; r, s = g.step(s) {
		next, ok := dfa.next[r]
		if ok {
			dfa = next
			continue
		}
		if len(dfa.list) == 0 && len(dfa.asteriskNexts) == 0 {
			g.dfaPool.Put(startPtr)
			return false
		}
		var (
			nlist         []*state
			asteriskNexts = dfa.asteriskNexts
		)
		for _, states := range [][]*state{dfa.list, dfa.asteriskNexts} {
			for _, state := range states {
				if state.match(r) {
					if state.next.kind == asteriskKind {
						// The last asterisk matches all characters, so it's a match.
						if state.next.next.kind == matchedKind {
							g.dfaPool.Put(startPtr)
							return true
						}
						asteriskNexts = append(asteriskNexts, state.next.next)
						continue
					}
					nlist = append(nlist, state.next)
				}
			}
		}
		if len(nlist) == 0 && len(dfa.list) == 0 && len(asteriskNexts) == len(dfa.asteriskNexts) {
			dfa.next[r] = dfa
			continue
		}
		next = &dfaState{list: nlist, asteriskNexts: asteriskNexts, next: make(map[rune]*dfaState)}
		dfa.next[r] = next
		dfa = next
	}

	for _, st := range dfa.list {
		if (st.kind == asteriskKind && st.next.kind == matchedKind) ||
			st.kind == matchedKind {
			g.dfaPool.Put(startPtr)
			return true
		}
	}
	g.dfaPool.Put(startPtr)
	return false
}

// Match returns whether the given string matches compiled glob.
func (g *Glob) Match(s string) bool {
	if g.onePassState != nil {
		return g.onePassMatch(s)
	}
	return g.dfaMatch(s)
}

// String implements fmt.Stringer.
func (g *Glob) String() string {
	return g.str
}
