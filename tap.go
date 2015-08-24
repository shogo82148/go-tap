package tap

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var ErrUnsupportedVersion = errors.New("tap: unsupported version")

const (
	DefaultTAPVersion = 12
)

// A TAP-Directive (Todo/Skip)
type Directive int

const (
	None Directive = iota // No directive given
	Todo                  // Testpoint is a TODO
	Skip                  // Testpoint was skipped
)

func (d Directive) String() string {
	switch d {
	case None:
		return "None"
	case Todo:
		return "TODO"
	case Skip:
		return "SKIP"
	}
	return ""
}

// A single TAP-Testline
type Testline struct {
	Ok          bool          // Whether the Testpoint executed ok
	Num         int           // The number of the test
	Description string        // A short description
	Directive   Directive     // Whether the test was skipped or is a todo
	Explanation string        // A short explanation why the test was skipped/is a todo
	Diagnostic  string        // A more detailed diagnostic message about the failed test
	Time        time.Duration // Time it took to test
	Yaml        []byte        // The inline Yaml-document, if given
}

// The outcome of a Testsuite
type Testsuite struct {
	Ok      bool          // Whether the Testsuite as a whole succeded
	Tests   []*Testline   // Description of all Testlines
	plan    int           // Number of tests intended to run
	version int           // version number of TAP
	Time    time.Duration // Time it took to test
}

// Parses TAP
type Parser struct {
	scanner      *bufio.Scanner
	lastNum      int
	suite        Testsuite
	startAt      time.Time
	lastExecTime time.Time
}

func NewParser(r io.Reader) (*Parser, error) {
	now := time.Now()
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, io.EOF
	}
	return &Parser{
		scanner:      scanner,
		lastNum:      0,
		startAt:      now,
		lastExecTime: now,
		suite: Testsuite{
			Ok:      true,
			Tests:   []*Testline{},
			plan:    0,
			version: DefaultTAPVersion,
		},
	}, nil
}

func (p *Parser) Next() (*Testline, error) {
	t := &Testline{}

	for {
		line := p.scanner.Text()

		// version
		if strings.HasPrefix(line, "TAP version ") {
			version, err := strconv.Atoi(line[len("TAP version "):])
			if err != nil {
				return nil, err
			}
			if version != 13 {
				return nil, ErrUnsupportedVersion
			}
			p.suite.version = version
			if !p.scanner.Scan() {
				return nil, io.EOF
			}
			continue
		}

		// plan
		if strings.HasPrefix(line, "1..") {
			start := len("1..")
			end := start
			for end < len(line) && unicode.IsDigit(rune(line[end])) {
				end++
			}
			plan, err := strconv.Atoi(line[start:end])
			if err != nil {
				return nil, err
			}
			p.suite.plan = plan
			if !p.scanner.Scan() {
				return nil, io.EOF
			}
			continue
		}

		// test
		if strings.HasPrefix(line, "ok ") {
			t = p.parseTestLine(true, line[len("ok "):])
			break
		}
		if strings.HasPrefix(line, "not ok ") {
			t = p.parseTestLine(false, line[len("not ok "):])
			break
		}

		// unknown line. skip it...

		if !p.scanner.Scan() {
			return nil, io.EOF
		}
	}

	p.suite.Tests = append(p.suite.Tests, t)
	return t, nil
}

func (p *Parser) Suite() (*Testsuite, error) {
	for {
		_, err := p.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	p.suite.Time = p.lastExecTime.Sub(p.startAt)
	for _, t := range p.suite.Tests {
		if !t.Ok {
			p.suite.Ok = false
		}
	}

	if len(p.suite.Tests) != p.suite.plan {
		p.suite.Ok = false
		return &p.suite, nil
	}

	return &p.suite, nil
}

func (p *Parser) parseTestLine(ok bool, line string) *Testline {
	// calculate time it took to test
	now := time.Now()
	duration := now.Sub(p.lastExecTime)
	p.lastExecTime = now

	index := 0

	// parse test number
	for index < len(line) && unicode.IsSpace(rune(line[index])) {
		index++
	}
	startNumber := index
	for index < len(line) && unicode.IsDigit(rune(line[index])) {
		index++
	}
	endNumber := index
	num := p.lastNum + 1
	if startNumber != endNumber {
		num, _ = strconv.Atoi(line[startNumber:endNumber])
	}
	p.lastNum = num

	// parse description & directive
	description := ""
	directiveStr := ""
	startDirective := strings.IndexRune(line[index:], '#')
	if startDirective >= 0 {
		startDirective += index
		description = strings.TrimSpace(line[index:startDirective])
		directiveStr = strings.TrimSpace(line[startDirective+1:])
	} else {
		description = strings.TrimSpace(line[index:])
	}

	directive := None
	explanation := directiveStr
	if len(directiveStr) > 4 && strings.EqualFold(directiveStr[0:4], "TODO") {
		directive = Todo
		explanation = strings.TrimSpace(directiveStr[4:])
	}
	if len(directiveStr) > 4 && strings.EqualFold(directiveStr[0:4], "SKIP") {
		directive = Skip
		explanation = strings.TrimSpace(directiveStr[4:])
	}

	// parse diagnostics
	diagnostics := []string{}
	for p.scanner.Scan() {
		text := p.scanner.Text()
		if len(text) == 1 || text[0] != '#' {
			break
		}
		diagnostics = append(diagnostics, strings.TrimSpace(text[1:])+"\n")
	}

	return &Testline{
		Ok:          ok,
		Num:         num,
		Description: description,
		Directive:   directive,
		Explanation: explanation,
		Diagnostic:  strings.Join(diagnostics, ""),
		Time:        duration,
	}
}

func (t *Testline) String() string {
	str := []string{}
	if t.Ok {
		str = append(str, "ok ")
	} else {
		str = append(str, "not ok ")
	}
	str = append(str, strconv.FormatInt(int64(t.Num), 10))

	if t.Description != "" {
		str = append(str, " ", t.Description)
	}

	if t.Directive != None {
		str = append(str, " # ", t.Directive.String())
		if t.Explanation != "" {
			str = append(str, " ", t.Explanation)
		}
	}

	return strings.Join(str, "")
}
