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

func TestSubtests(t *testing.T) {
	r := strings.NewReader(`ok 1 - foo
    # Subtest: bar
    ok 1 - subtest1
    ok 2 - subtest2 # TODO not implemented yet
    # note message for subtest2
    ok 3 - subtest3
    1..3
ok 2 - bar
ok 3 - foobar
1..3
`)
	p, err := NewParser(r)
	if err != nil {
		panic(err)
	}

	suite, err := p.Suite()
	if err != nil {
		panic(err)
	}

	if len(suite.Tests) != 3 {
		t.Errorf("want 3\ngot %d", len(suite.Tests))
	}
	if suite.Tests[0].SubTests != nil {
		t.Errorf("want no subtests\ngot %v", suite.Tests[0].SubTests)
	}
	if len(suite.Tests[1].SubTests) != 3 {
		t.Errorf("want 3\ngot %d", len(suite.Tests[0].SubTests))
	}
	if got, want := suite.Tests[1].SubTests[0].String(), "ok 1 - subtest1"; got != want {
		t.Errorf("want %s\ngot %s", want, got)
	}
	if got, want := suite.Tests[1].SubTests[1].String(), "ok 2 - subtest2 # TODO not implemented yet"; got != want {
		t.Errorf("want %s\ngot %s", want, got)
	}
	if got, want := suite.Tests[1].SubTests[2].String(), "ok 3 - subtest3"; got != want {
		t.Errorf("want %s\ngot %s", want, got)
	}
	if suite.Tests[2].SubTests != nil {
		t.Errorf("want no subtests\ngot %v", suite.Tests[2].SubTests)
	}
}

func TestSubsubtests(t *testing.T) {
	r := strings.NewReader(`ok 1 - foo
    # Subtest: bar
        # Subtest: subtest1
        ok 1 - subsubtest1
        ok 2 - subsubtest2 # TODO not implemented yet
        # note message for subsubtest2
        ok 3 - subsubtest3
        1..3
    ok 1 - subtest1
    1..1
ok 2 - bar
ok 3 - foobar
1..3
`)
	p, err := NewParser(r)
	if err != nil {
		panic(err)
	}

	suite, err := p.Suite()
	if err != nil {
		panic(err)
	}

	if len(suite.Tests) != 3 {
		t.Errorf("want 3\ngot %d", len(suite.Tests))
	}
	if suite.Tests[0].SubTests != nil {
		t.Errorf("want no subtests\ngot %v", suite.Tests[0].SubTests)
	}
	if len(suite.Tests[1].SubTests) != 1 {
		t.Errorf("want 1\ngot %d", len(suite.Tests[0].SubTests))
	}
	if got, want := suite.Tests[1].SubTests[0].String(), "ok 1 - subtest1"; got != want {
		t.Errorf("want %s\ngot %s", want, got)
	}
	if len(suite.Tests[1].SubTests[0].SubTests) != 3 {
		t.Errorf("want 3\ngot %d", len(suite.Tests[1].SubTests[0].SubTests))
	}
}

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(`1..3
ok 1 hogehoge
not ok foobar
# Doesn't wiggle
not ok 3 foobar # TODO not implemented yet`)
		p, _ := NewParser(r)
		p.Suite()
	}
}
