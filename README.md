go-tap
====

TAP (Test Anything Protocol) parser in golang.

``` go
import "github.com/shogo82148/go-tap"

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

	for _, t := range suite.Tests {
		fmt.Println(t)
	}

	// Output:
	// ok 1 hogehoge
	// not ok 2 foobar
	// not ok 3 foobar # TODO not implemented yet
}
```

see [godoc](https://godoc.org/github.com/shogo82148/go-tap) for more detail.
