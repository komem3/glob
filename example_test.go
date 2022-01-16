package glob_test

import (
	"fmt"

	"github.com/komem3/glob"
)

func Example() {
	matcher := glob.MustCompile("Hello *d")
	fmt.Printf("%t", matcher.MatchString("Hello World"))
	// Output: true
}
