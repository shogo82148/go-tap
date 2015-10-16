package tap

import (
	"fmt"
	"strings"
	"testing"
)

func ExampleNewParser() {
	r := strings.NewReader(`1..3
ok 1 hogehoge
not ok foobar
# Doesn't wiggle
not ok 3 foobar # TODO not implemented yet`)
	p, err := NewParser(r)
	if err != nil {
		panic(err)
	}

	suite, err := p.Suite()
	if err != nil {
		panic(err)
	}
	if suite.Ok {
		fmt.Println("Everything ok")
		return
	}

	for _, t := range suite.Tests {
		fmt.Println(t)
	}

	// Output:
	// ok 1 hogehoge
	// not ok 2 foobar
	// not ok 3 foobar # TODO not implemented yet
}

func TestYAML(t *testing.T) {
	r := strings.NewReader(`TAP version 13
ok 1 - YAML TEST
    ---
    - foo
    - bar
    ...
1..1`)
	p, err := NewParser(r)
	if err != nil {
		panic(err)
	}

	suite, err := p.Suite()
	if err != nil {
		panic(err)
	}

	if string(suite.Tests[0].Yaml) != "    - foo\n    - bar\n" {
		t.Errorf("want %s got %s", "    - foo\n    - bar\n", string(suite.Tests[0].Yaml))
	}
}
