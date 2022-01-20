# glob

[![Go Reference](https://pkg.go.dev/badge/github.com/komem3/globg.svg)](https://pkg.go.dev/github.com/komem3/glob)

Package glob implements glob pattern match.
This is implemented according to [IEEE Std 1003.1-2017](https://pubs.opengroup.org/onlinepubs/9699919799.2018edition/).

## Special Chars

- `?`: A `<question-mark>` is a pattern that shall match any character.
- `*`: An `<asterisk>` is a pattern that shall match multiple characters, as described in Patterns Matching Multiple Characters.
- `[`: If an open bracket introduces a bracket expression as in RE Bracket Expression.

## Usage

Provides same interface with the standard library regexp.

```go
package main

import (
	"fmt"

	"github.com/komem3/glob"
)

func main() {
	matcher := glob.MustCompile("Hello *d")
	fmt.Printf("%t", matcher.MatchString("Hello World"))
	// Output: true
}
```

## Benchmark (glob vs regexp)

```
goos: linux
goarch: amd64
pkg: github.com/komem3/glob
cpu: 11th Gen Intel(R) Core(TM) i7-1165G7 @ 2.80GHz
BenchmarkGlob_Match/full_string[String]/glob-8    	 2097278	       525.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/full_string[String]/regex-8   	  121653	     14700 ns/op	       7 B/op	       0 allocs/op
BenchmarkGlob_Match/front_asterisk[String]/glob-8 	   17473	     67045 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/front_asterisk[String]/regex-8         	    1305	    839839 ns/op	     655 B/op	       0 allocs/op
BenchmarkGlob_Match/last_asterisk[String]/glob-8           	 7118701	       146.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/last_asterisk[String]/regex-8          	  390000	      3459 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/equal[String]/glob-8                   	16870214	        81.77 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/equal[String]/regex-8                  	11177442	       108.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/forward[String]/glob-8                 	 9613209	       145.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/forward[String]/regex-8                	 6637020	       182.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/backward[String]/glob-8                	  175374	      6596 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/backward[String]/regex-8               	  133812	      7701 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/partial(no_prefix)[String]/glob-8      	  744122	      1494 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/partial(no_prefix)[String]/regex-8     	    5305	    213007 ns/op	     161 B/op	       0 allocs/op
BenchmarkGlob_Match/partial(prefix)[String]/glob-8         	 3426975	       352.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/partial(prefix)[String]/regex-8        	    6175	    194106 ns/op	     138 B/op	       0 allocs/op
BenchmarkGlob_Match/one_pass[String]/glob-8                	11936358	        84.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/one_pass[String]/regex-8               	11558540	       115.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkGlob_Match/full_string[Reader]/glob-8             	  424182	      2502 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/full_string[Reader]/regex-8            	   49975	     24444 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/front_asterisk[Reader]/glob-8          	   10000	    111649 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/front_asterisk[Reader]/regex-8         	    1032	   1043365 ns/op	      39 B/op	       1 allocs/op
BenchmarkGlob_Match/last_asterisk[Reader]/glob-8           	 5174074	       218.5 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/last_asterisk[Reader]/regex-8          	 3310730	       397.5 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/equal[Reader]/glob-8                   	 6150862	       183.4 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/equal[Reader]/regex-8                  	 5147601	       199.7 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/forward[Reader]/glob-8                 	 5149942	       216.2 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/forward[Reader]/regex-8                	 4580384	       253.4 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/backward[Reader]/glob-8                	   11835	    116044 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/backward[Reader]/regex-8               	    3776	    295302 ns/op	      33 B/op	       1 allocs/op
BenchmarkGlob_Match/partial(no_prefix)[Reader]/glob-8      	  325057	      3698 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/partial(no_prefix)[Reader]/regex-8     	   44877	     24113 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/partial(prefix)[Reader]/glob-8         	  428636	      3290 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/partial(prefix)[Reader]/regex-8        	   52003	     21867 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/one_pass[Reader]/glob-8                	 7638493	       156.8 ns/op	      32 B/op	       1 allocs/op
BenchmarkGlob_Match/one_pass[Reader]/regex-8               	 6075450	       182.6 ns/op	      32 B/op	       1 allocs/op
```

## License

MIT
