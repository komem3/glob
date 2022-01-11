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
	"io"
	"regexp"
	"sync"
	"unicode/utf8"
)

type steper interface {
	step(index int) (rune, int)
}

type stringStep struct {
	str string
}

type byteStep struct {
	bs []byte
}

type readerStep struct {
	reader io.RuneReader
}

func (s *stringStep) step(index int) (rune, int) {
	if len(s.str) > index {
		c := s.str[index]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRuneInString(s.str[index:])
	}
	return utf8.RuneError, 0
}

func (b *byteStep) step(index int) (rune, int) {
	if len(b.bs) > index {
		c := b.bs[index]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRune(b.bs[index:])
	}
	return utf8.RuneError, 0
}

func (rs *readerStep) step(_ int) (rune, int) {
	r, size, err := rs.reader.ReadRune()
	if err != nil {
		return utf8.RuneError, size
	}
	return r, size
}

var stringStepPool = sync.Pool{
	New: func() interface{} {
		return new(stringStep)
	},
}

var byteStepPool = sync.Pool{
	New: func() interface{} {
		return new(byteStep)
	},
}

var readerStepPool = sync.Pool{
	New: func() interface{} {
		return new(readerStep)
	},
}

var backTrackPool = sync.Pool{
	New: func() interface{} {
		return new(posStack)
	},
}

var dfaPool = sync.Pool{
	New: func() interface{} {
		return new(dfaState)
	},
}

type kind int

const (
	matchedKind = iota + 1
	runeKind
	asteriskKind
	questionKind
	regexKind
)

type algorithm int

const (
	nfa algorithm = iota + 1
	backtrack
)

type state struct {
	kind kind
	next *state

	r  rune
	re *regexp.Regexp
}

type dfaState struct {
	list          []*state
	asteriskNexts []*state
}

type posStack struct {
	index []int
	state []*state
}

func (n *dfaState) reset() {
	n.list = n.list[:0]
	n.asteriskNexts = n.asteriskNexts[:0]
}

func (p *posStack) push(index int, state *state) {
	p.index = append(p.index, index)
	p.state = append(p.state, state)
}

func (p *posStack) reset() {
	p.index = p.index[:0]
	p.state = p.state[:0]
}

func (p *posStack) pop() (int, *state) {
	if len(p.index) == 0 {
		return -1, nil
	}
	lastIndex := p.index[len(p.index)-1]
	lastaState := p.state[len(p.state)-1]
	p.index = p.index[:len(p.index)-1]
	p.state = p.state[:len(p.state)-1]
	return lastIndex, lastaState
}

// Glob has compiled pattern.
type Glob struct {
	str string

	algorithm  algorithm
	startState *state
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
	var (
		runes        = []rune(pattern)
		escape       bool
		patternState = &state{}
		startState   = patternState
		asterisks    int
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
			asterisks++
		case !escape && runes[i] == '?':
			patternState.next = &state{kind: questionKind}
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
			patternState.next = &state{re: regex, kind: regexKind}
			patternState = patternState.next
			i += end
		default:
			patternState.next = &state{r: runes[i], kind: runeKind}
			patternState = patternState.next
		}
		escape = false
	}
	patternState.next = &state{kind: matchedKind}

	glob := &Glob{
		str:        pattern,
		startState: startState.next,
	}

	// DFA approach requires o(n^2) times and n memory allocate.
	// Backtrack approach requires o(n^2*asterisks) and asterisks memory allocate.
	// So, use backtrack if pattern have less than two asterisks.
	if asterisks < 2 {
		glob.algorithm = backtrack
	} else {
		glob.algorithm = nfa
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

func (g *Glob) dfaMatch(steper steper) bool {
	// pattern is only one '*'.
	if g.startState.kind == asteriskKind && g.startState.next.kind == matchedKind {
		return true
	}

	dfa := dfaPool.Get().(*dfaState)
	defer func() {
		dfa.reset()
		dfaPool.Put(dfa)
	}()

	if g.startState.kind == asteriskKind {
		dfa.asteriskNexts = append(dfa.asteriskNexts, g.startState.next)
	} else {
		dfa.list = append(dfa.list, g.startState)
	}

	var index int
	for {
		r, size := steper.step(index)
		if r == utf8.RuneError {
			break
		}
		index += size

		list := dfa.list[:]
		dfa.list = dfa.list[:0]
		for _, stateList := range [2][]*state{list, dfa.asteriskNexts[:]} {
			for _, state := range stateList {
				if state.match(r) {
					if state.next.kind == asteriskKind {
						// The last asterisk matches all characters, so it's a match.
						if state.next.next.kind == matchedKind {
							return true
						}
						dfa.asteriskNexts = append(dfa.asteriskNexts, state.next.next)
						continue
					}
					dfa.list = append(dfa.list, state.next)
				}
			}
		}
		if len(dfa.asteriskNexts) == 0 && len(dfa.list) == 0 {
			return false
		}
	}

	for _, st := range dfa.list {
		if (st.kind == asteriskKind && st.next.kind == matchedKind) ||
			st.kind == matchedKind {
			return true
		}
	}
	return false
}

func (g *Glob) backtrack(steper steper) bool {
	stack := backTrackPool.Get().(*posStack)
	defer func() {
		stack.reset()
		backTrackPool.Put(stack)
	}()

	stack.push(0, g.startState)
	for index, state := stack.pop(); state != nil; index, state = stack.pop() {
		for {
			r, size := steper.step(index)
			if r == utf8.RuneError {
				if state.kind == matchedKind ||
					state.kind == asteriskKind && state.next.kind == matchedKind {
					return true
				}
				break
			}
			index += size

			if state.kind == asteriskKind {
				if state.next.kind == matchedKind {
					return true
				}
				if state.next.match(r) {
					stack.push(index, state)
					state = state.next.next
				}
				continue
			}
			if state.match(r) {
				state = state.next
				continue
			}
			break
		}
	}
	return false
}

// Match returns whether the given bytes matches compiled glob.
func (g *Glob) Match(bs []byte) bool {
	steper := byteStepPool.Get().(*byteStep)
	steper.bs = bs
	defer func() {
		byteStepPool.Put(steper)
	}()
	if g.algorithm == backtrack {
		return g.backtrack(steper)
	}
	return g.dfaMatch(steper)
}

// MatchString returns whether the given string matches compiled glob.
func (g *Glob) MatchString(s string) bool {
	steper := stringStepPool.Get().(*stringStep)
	steper.str = s
	defer func() {
		stringStepPool.Put(steper)
	}()
	if g.algorithm == backtrack {
		return g.backtrack(steper)
	}
	return g.dfaMatch(steper)
}

// MatchReader returns whether the given reader matches compiled glob.
func (g *Glob) MatchReader(reader io.RuneReader) bool {
	steper := readerStepPool.Get().(*readerStep)
	steper.reader = reader
	defer func() {
		readerStepPool.Put(steper)
	}()
	return g.dfaMatch(steper)
}

// String implements fmt.Stringer.
func (g *Glob) String() string {
	return g.str
}

func (s *state) match(r rune) bool {
	switch s.kind {
	case runeKind:
		return s.r == r
	case regexKind:
		return s.re.MatchString(string(r))
	case questionKind:
		return true
	case matchedKind:
		return false
	default:
		panic("unexpected kind")
	}
}
