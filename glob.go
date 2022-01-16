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
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

type steper interface {
	step(index int, state *state) (r rune, next int)
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

func (s *stringStep) step(index int, state *state) (rune, int) {
	if len(s.str) > index {
		if state != nil && state.strPrefix != "" {
			prefixIndex := strings.Index(s.str[index:], state.strPrefix)
			if prefixIndex == -1 {
				return utf8.RuneError, -1
			}
			index += prefixIndex
			r, size := utf8.DecodeRuneInString(s.str[index:])
			return r, index + size
		}
		r, size := utf8.DecodeRuneInString(s.str[index:])
		return r, index + size
	}
	return utf8.RuneError, -1
}

func (b *byteStep) step(index int, state *state) (rune, int) {
	if len(b.bs) > index {
		if state != nil && state.bytesPrefix != nil {
			prefixIndex := bytes.Index(b.bs[index:], state.bytesPrefix)
			if prefixIndex == -1 {
				return utf8.RuneError, -1
			}
			index += prefixIndex
			r, size := utf8.DecodeRune(b.bs[index:])
			return r, index + size
		}
		r, size := utf8.DecodeRune(b.bs[index:])
		return r, index + size
	}
	return utf8.RuneError, -1
}

func (rs *readerStep) step(_ int, state *state) (rune, int) {
	if state != nil && state.kind == runeKind {
		for {
			r, size, err := rs.reader.ReadRune()
			if err != nil {
				return utf8.RuneError, -1
			}
			if r == state.r {
				return r, size
			}
		}
	}
	r, size, err := rs.reader.ReadRune()
	if err != nil {
		return utf8.RuneError, -1
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

var nfaPool = sync.Pool{
	New: func() interface{} {
		return new(nfaState)
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

type state struct {
	kind kind
	next *state

	strPrefix   string
	bytesPrefix []byte

	r  rune
	re *regexp.Regexp
}

type nfaState struct {
	list          []*state
	asteriskNexts *state
}

type posStack struct {
	index int
	state *state
}

func (n *nfaState) reset() {
	n.list = n.list[:0]
	n.asteriskNexts = nil
}

func (p *posStack) push(index int, state *state) {
	p.index = index
	p.state = state
}

func (p *posStack) reset() {
	p.index = 0
	p.state = nil
}

func (p *posStack) pop() (int, *state) {
	if p.state == nil {
		return -1, nil
	}
	index, state := p.index, p.state
	p.reset()
	return index, state
}

// Glob has compiled pattern.
type Glob struct {
	str        string
	startState *state

	onePass bool
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
		prefixState  *state
		prefixBuf    = new(bytes.Buffer)
	)
	prefixAdd := func() {
		if prefixState != nil {
			prefixState.strPrefix = prefixBuf.String()
			prefixState.bytesPrefix = prefixBuf.Bytes()
		}
		prefixState = nil
	}
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
			prefixAdd()
		case !escape && runes[i] == '?':
			patternState.next = &state{kind: questionKind}
			patternState = patternState.next
			prefixAdd()
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
			prefixAdd()
		default:
			patternState.next = &state{r: runes[i], kind: runeKind}
			patternState = patternState.next
			if prefixState == nil {
				prefixState = patternState
				prefixBuf = new(bytes.Buffer)
			}
			prefixBuf.WriteRune(runes[i])
		}
		escape = false
	}
	prefixAdd()
	patternState.next = &state{kind: matchedKind}

	glob := &Glob{
		str:        pattern,
		startState: startState.next,
		onePass:    asterisks == 0,
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

func (g *Glob) nfaMatch(steper steper) bool {
	// pattern is only one '*'.
	if g.startState.kind == asteriskKind && g.startState.next.kind == matchedKind {
		return true
	}

	nfa := nfaPool.Get().(*nfaState)
	defer func() {
		nfa.reset()
		nfaPool.Put(nfa)
	}()

	if g.startState.kind == asteriskKind {
		nfa.asteriskNexts = g.startState.next
		nfa.list = append(nfa.list, g.startState.next)
	} else {
		nfa.list = append(nfa.list, g.startState)
	}

	var (
		index   int
		r       rune
		matched = len(nfa.list) > 0 && nfa.list[0].kind == matchedKind
	)
	for {
		if !matched && nfa.asteriskNexts != nil && len(nfa.list) == 0 {
			r, index = steper.step(index, nfa.asteriskNexts)
		} else {
			r, index = steper.step(index, nil)
		}
		if r == utf8.RuneError {
			return matched
		}

		matched = false
		list := nfa.list[:]
		nfa.list = nfa.list[:0]
		for _, state := range list {
			if state.match(r) {
				if state.next.kind == asteriskKind {
					// The last asterisk matches all characters, so it's a match.
					if state.next.next.kind == matchedKind {
						return true
					}
					nfa.asteriskNexts = state.next.next
					nfa.list = nfa.list[:0]
					break
				}
				if state.next.kind == matchedKind {
					matched = true
					continue
				}
				nfa.list = append(nfa.list, state.next)
			}
		}
		if nfa.asteriskNexts != nil {
			nfa.list = append(nfa.list, nfa.asteriskNexts)
		}
		if !matched && nfa.asteriskNexts == nil && len(nfa.list) == 0 {
			return false
		}
	}
}

func (g *Glob) backtrack(steper steper) bool {
	stack := backTrackPool.Get().(*posStack)
	defer func() {
		stack.reset()
		backTrackPool.Put(stack)
	}()

	var r rune
	stack.push(0, g.startState)
	for index, state := stack.pop(); state != nil; index, state = stack.pop() {
		for {
			if state.kind == asteriskKind {
				r, index = steper.step(index, state.next)
			} else {
				r, index = steper.step(index, nil)
			}
			if r == utf8.RuneError {
				if state.kind == matchedKind ||
					state.kind == asteriskKind && state.next.kind == matchedKind {
					return true
				}
				break
			}

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

func (g *Glob) onePassMatch(steper steper) bool {
	var (
		r     rune
		index int
		state = g.startState
	)
	for {
		r, index = steper.step(index, nil)
		if r == utf8.RuneError {
			return state.kind == matchedKind
		}
		if state.match(r) {
			state = state.next
			continue
		}
		return false
	}
}

// Match returns whether the given bytes matches compiled glob.
func (g *Glob) Match(bs []byte) bool {
	steper := byteStepPool.Get().(*byteStep)
	steper.bs = bs
	defer func() {
		byteStepPool.Put(steper)
	}()
	if g.onePass {
		return g.onePassMatch(steper)
	}
	return g.backtrack(steper)
}

// MatchString returns whether the given string matches compiled glob.
func (g *Glob) MatchString(s string) bool {
	steper := stringStepPool.Get().(*stringStep)
	steper.str = s
	defer func() {
		stringStepPool.Put(steper)
	}()
	if g.onePass {
		return g.onePassMatch(steper)
	}
	return g.backtrack(steper)
}

// MatchReader returns whether the given reader matches compiled glob.
func (g *Glob) MatchReader(reader io.RuneReader) bool {
	steper := readerStepPool.Get().(*readerStep)
	steper.reader = reader
	defer func() {
		readerStepPool.Put(steper)
	}()
	if g.onePass {
		return g.onePassMatch(steper)
	}
	return g.nfaMatch(steper)
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
