# glob

[![Go Reference](https://pkg.go.dev/badge/github.com/komem3/globg.svg)](https://pkg.go.dev/github.com/komem3/globg)

Package glob implements glob pattern match.
This is implemented according to [IEEE Std 1003.1-2017](https://pubs.opengroup.org/onlinepubs/9699919799.2018edition/).

## Special Chars

'?' A <question-mark> is a pattern that shall match any character.
'\*' An <asterisk> is a pattern that shall match multiple characters, as described in Patterns Matching Multiple Characters.
'[' If an open bracket introduces a bracket expression as in RE Bracket Expression. See [regexp/syntax](https://pkg.go.dev/regexp/syntax).

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

## License

MIT
