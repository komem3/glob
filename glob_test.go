package glob_test

import (
	"math/rand"
	"regexp"
	"testing"

	"github.com/komem3/glob"
)

const globText = "baaabab"

var globTestPattern = []struct {
	pattern string
	match   bool
}{
	// match pattern
	{"baaabab", true},
	{"b***bab", true},
	{"*****ba*****ab", true},
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
	t.Parallel()
	for _, tt := range globTestPattern {
		tt := tt
		t.Run(tt.pattern, func(t *testing.T) {
			t.Parallel()
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.Match(globText); match != tt.match {
				t.Fatalf("first Match want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match(globText); match != tt.match {
				t.Fatalf("second Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

func TestGlob_MatchMultiByte(t *testing.T) {
	t.Parallel()
	const pattern = "Hello 世界"
	for _, tt := range []struct {
		pattern string
		match   bool
	}{
		{"*世界", true},
		{"*世界*", true},
		{"Hello*", true},
		{"Hello**世界", true},
		{"Hello*??", true},
		{"Hello", false},
	} {
		tt := tt
		t.Run(tt.pattern, func(t *testing.T) {
			t.Parallel()
			matcher, err := glob.Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if match := matcher.Match(pattern); match != tt.match {
				t.Fatalf("first Match want %t, but got %t ", tt.match, match)
			}
			if match := matcher.Match(pattern); match != tt.match {
				t.Fatalf("second Match want %t, but got %t ", tt.match, match)
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
	{"last asterisk", "*a*a*", "a.*a"},
	{"equal", "aaaaaaaa", "^aaaaaaaa$"},
	{"backward", "a*", "^a"},
	{"forward", "*a", "a$"},
	{"partial", "*a*", "a"},
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
					matcher.Match(randStr[i%randNum])
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
