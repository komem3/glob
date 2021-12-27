package glob_test

import (
	"math/rand"
	"regexp"
	"testing"
	"time"

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
				t.Errorf("Match want %t, but got %t ", tt.match, match)
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
				t.Errorf("Match want %t, but got %t ", tt.match, match)
			}
		})
	}
}

const randNum = 1000

func randSeq() []string {
	rand.Seed(time.Now().UnixNano())
	b := make([]string, 0, randNum)
	for i := 0; i < randNum; i++ {
		rb := make([]byte, 1000)
		rand.Read(rb)
		b = append(b, string(rb))
	}
	return b
}

func BenchmarkGlob_Match(b *testing.B) {
	randStr := randSeq()
	matcher, err := glob.Compile("*a*a*")
	if err != nil {
		b.Fatal(err)
	}
	regex, err := regexp.Compile("a.*a")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.Run("glob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matcher.Match(randStr[i%randNum])
		}
	})
	b.ResetTimer()
	b.Run("regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			regex.MatchString(randStr[i%randNum])
		}
	})
}

func BenchmarkGlob_Match_Equal(b *testing.B) {
	randStr := randSeq()
	matcher, err := glob.Compile("aaaaaaaa")
	if err != nil {
		b.Fatal(err)
	}
	regex, err := regexp.Compile("aaaaaaaa")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.Run("glob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matcher.Match(randStr[i%randNum])
		}
	})
	b.ResetTimer()
	b.Run("regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			regex.MatchString(randStr[i%randNum])
		}
	})
}

func BenchmarkGlob_Match_Backward(b *testing.B) {
	randStr := randSeq()
	matcher, err := glob.Compile("a*")
	if err != nil {
		b.Fatal(err)
	}
	regex, err := regexp.Compile("^a")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.Run("glob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matcher.Match(randStr[i%randNum])
		}
	})
	b.ResetTimer()
	b.Run("regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			regex.MatchString(randStr[i%randNum])
		}
	})
}

func BenchmarkGlob_Match_Forward(b *testing.B) {
	randStr := randSeq()
	matcher, err := glob.Compile("*a")
	if err != nil {
		b.Fatal(err)
	}
	regex, err := regexp.Compile("a$")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.Run("glob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matcher.Match(randStr[i%randNum])
		}
	})
	b.ResetTimer()
	b.Run("regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			regex.MatchString(randStr[i%randNum])
		}
	})
}

func BenchmarkGlob_Match_ForwardBackwardk(b *testing.B) {
	randStr := randSeq()
	matcher, err := glob.Compile("*a*")
	if err != nil {
		b.Fatal(err)
	}
	regex, err := regexp.Compile("a")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.Run("glob", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matcher.Match(randStr[i%randNum])
		}
	})
	b.ResetTimer()
	b.Run("regex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			regex.MatchString(randStr[i%randNum])
		}
	})
}
