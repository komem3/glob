package glob_test

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/komem3/glob"
)

const globText = "baaabab"

type testCase = struct {
	pattern string
	match   bool
}

var globTestPattern = []testCase{
	// match pattern
	{"*", true},
	{"baaabab", true},
	{"b***bab", true},
	{"*****ba*****ab", true},
	{"**b*b*ab", true},
	{"*ab", true},
	{"**ab", true},
	{"*baaabab", true},
	{"ba*", true},
	{"ba**", true},
	{"*ab*", true},
	{"**aaaba**", true},
	{"baaabab*", true},
	{"baa??ab", true},
	{"b*a?", true},
	{"b*b", true},
	{"?*", true},
	{"?a*ba?", true},
	{"??*??", true},
	{"[ab][^c]aaba[[:alnum:]]", true},
	// mismatch pattern
	{"a", false},
	{"b**a", false},
	{"**a", false},
	{"a*", false},
	{"*c*", false},
	{"baa", false},
	{"baaaba?b", false},
	{"bab", false},
	{"????", false},
	{"[A-Z]*", false},
}

func TestGlob_Match(t *testing.T) {
	for _, tt := range globTestPattern {
		t.Run(tt.pattern, func(t *testing.T) {
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.MatchString(globText); match != tt.match {
				t.Fatalf("MatchString want %t, but got %t ", tt.match, match)
			}
			if match := matcher.MatchReader(strings.NewReader(globText)); match != tt.match {
				t.Fatalf("MatchReader want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match([]byte(globText)); match != tt.match {
				t.Fatalf("Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

func TestGlob_MatchOne(t *testing.T) {
	const char = "a"
	for _, tt := range []testCase{
		{"a", true},
		{"b", false},
		{"*", true},
		{"?", true},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.MatchString(char); match != tt.match {
				t.Fatalf("MatchString want %t, but got %t ", tt.match, match)
			}
			if match := matcher.MatchReader(strings.NewReader(char)); match != tt.match {
				t.Fatalf("MatchReader want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match([]byte(char)); match != tt.match {
				t.Fatalf("Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

func TestGlob_MatchEmpty(t *testing.T) {
	const empty = ""
	for _, tt := range []testCase{
		{"*", true},
		{"", true},
		{"a", false},
		{"?", false},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.MatchString(empty); match != tt.match {
				t.Fatalf("MatchString want %t, but got %t ", tt.match, match)
			}
			if match := matcher.MatchReader(strings.NewReader(empty)); match != tt.match {
				t.Fatalf("MatchReader want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match([]byte(empty)); match != tt.match {
				t.Fatalf("Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

func TestGlob_MatchMultiByte(t *testing.T) {
	const pattern = "Hello 世界"
	for _, tt := range []testCase{
		{"*世界", true},
		{"*世界*", true},
		{"Hello*", true},
		{"Hello**世界", true},
		{"Hello*??", true},
		{"Hello", false},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.MatchString(pattern); match != tt.match {
				t.Fatalf("MatchString want %t, but got %t ", tt.match, match)
			}
			if match := matcher.MatchReader(strings.NewReader(pattern)); match != tt.match {
				t.Fatalf("MatchReader want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match([]byte(pattern)); match != tt.match {
				t.Fatalf("Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

var (
	randNum     = 10000
	letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func randSeq() []string {
	b := make([]string, randNum)
	for i := 0; i < len(b); i++ {
		b[i] = randomString(randNum)
	}
	return b
}

var benchTests = []struct {
	name         string
	globPattern  string
	regexPattern string
}{
	{"full string", "a*a*ab*a", "^a.*a.*ab.*a$"},
	{"front asterisk", "*?a*a", ".a.*a$"},
	{"last asterisk", "a*a*", "^a.*a"},
	{"equal", "aaaaaaaa", "^aaaaaaaa$"},
	{"forward", "a*", "^a"},
	{"backward", "*a", "a$"},
	{"partial(no prefix)", "*?a*", ".a"},
	{"partial(prefix)", "*a*", "a"},
	{"one pass", "a??a", "^a..a$"},
}

func BenchmarkGlob_Match(b *testing.B) {
	randStr := randSeq()
	for _, tt := range benchTests {
		b.Run(tt.name, func(b *testing.B) {
			b.Run("glob", func(b *testing.B) {
				matcher := glob.MustCompile(tt.globPattern)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					matcher.MatchString(randStr[i%randNum])
				}
			})
			b.Run("regex", func(b *testing.B) {
				matcher := regexp.MustCompile(tt.regexPattern)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					matcher.MatchString(randStr[i%randNum])
				}
			})
		})
	}
}
