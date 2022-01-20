package glob

import (
	"fmt"
	"log"
)

type bracket struct {
	nonMatch   bool
	charRanges []*charRange
	runes      []rune
}

type charRange struct {
	startPoint rune
	endPoint   rune
}

// newBracket create bracket.
// pattern is expected "[regex bracket] pattern."
//
// example:
//	newBracket("[a-zA-Z]")
//	newBracket("[[:alpha:]")
func newBracket(pattern []rune) (*bracket, error) {
	log.Printf("%s", string(pattern))
	if len(pattern) < 2 {
		return nil, fmt.Errorf("%s is expected '[regex bracket]'", string(pattern))
	}
	pattern = pattern[1 : len(pattern)-1]
	if len(pattern) == 0 {
		return nil, fmt.Errorf("%s is expected '[regex bracket]'", string(pattern))
	}

	bracket := new(bracket)
	for i := 0; i < len(pattern); i++ {
		r := pattern[i]
		if r == '^' && i == 0 {
			bracket.nonMatch = true
			continue
		}
		if r == '[' {
			closeIndex := indexCloseSquare(pattern[i:])
			if closeIndex == -1 {
				return nil, fmt.Errorf("there is no ']' corresponding to '['")
			}
			rang, rs, err := characterClass(pattern[i : i+closeIndex+1])
			if err != nil {
				return nil, err
			}
			if len(rang) != 0 {
				bracket.charRanges = append(bracket.charRanges, rang...)
			}
			if len(rs) != 0 {
				bracket.runes = append(bracket.runes, rs...)
			}
			i += closeIndex
			continue
		}
		if i+2 < len(pattern) && pattern[i+1] == '-' {
			bracket.charRanges = append(bracket.charRanges, &charRange{
				startPoint: pattern[i],
				endPoint:   pattern[i+2],
			})
			i += 2
			continue
		}
		bracket.runes = append(bracket.runes, r)
	}
	return bracket, nil
}

func characterClass(pattern []rune) ([]*charRange, []rune, error) {
	switch string(pattern) {
	case "[:alnum:]":
		return []*charRange{
			{startPoint: '0', endPoint: '9'},
			{startPoint: 'a', endPoint: 'z'},
			{startPoint: 'A', endPoint: 'Z'},
		}, nil, nil
	case "[:alpha:]":
		return []*charRange{
			{startPoint: 'a', endPoint: 'z'},
			{startPoint: 'A', endPoint: 'Z'},
		}, nil, nil
	case "[:blank:]":
		return []*charRange{
			{startPoint: '\t', endPoint: '\t'},
		}, []rune{' '}, nil
	case "[:cntrl:]":
		return []*charRange{
			{startPoint: '\x00', endPoint: '\x1F'},
		}, []rune{'\x7F'}, nil
	case "[:digit:]":
		return []*charRange{
			{startPoint: '0', endPoint: '9'},
		}, nil, nil
	case "[:graph:]":
		return []*charRange{
			{startPoint: '!', endPoint: '~'},
		}, nil, nil
	case "[:lower:]":
		return []*charRange{
			{startPoint: 'a', endPoint: 'z'},
		}, nil, nil
	case "[:print:]":
		return []*charRange{
			{startPoint: ' ', endPoint: '~'},
		}, nil, nil
	case "[:punct:]":
		return nil, []rune{'!', '-', '/', ':', '-', '@', '[', '-', '`', '{', '-', '~'}, nil
	case "[:space:]":
		return nil, []rune{'\t', '\n', '\v', '\f', '\r', ' '}, nil
	case "[:upper:]":
		return []*charRange{
			{startPoint: 'A', endPoint: 'Z'},
		}, nil, nil
	case "[:xdigit:]":
		return []*charRange{
			{startPoint: '0', endPoint: '9'},
			{startPoint: 'a', endPoint: 'f'},
			{startPoint: 'A', endPoint: 'F'},
		}, nil, nil
	}
	return nil, nil, fmt.Errorf("unexpected syntax %s", string(pattern))
}

func (b *bracket) match(r rune) bool {
	for _, cr := range b.charRanges {
		if cr.startPoint <= r && cr.endPoint >= r {
			return !b.nonMatch
		}
	}
	for _, br := range b.runes {
		if br == r {
			return !b.nonMatch
		}
	}
	return b.nonMatch
}
