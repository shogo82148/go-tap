package tap

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrUnsupportedVersion is an error for unsupported TAP version.
var ErrUnsupportedVersion = errors.New("tap: unsupported version")

// DefaultTAPVersion is default TAP version.
const DefaultTAPVersion = 12

// Directive is a TAP-Directive (TODO/Skip)
type Directive int

const (
	// None describes no directive given.
	None Directive = iota

	// TODO describess the testpoint is a TODO.
	TODO

	// Skip describes the testpoint was skipped.
	Skip
)

func (d Directive) String() string {
	switch d {
	case None:
		return "None"
	case TODO:
		return "TODO"
	case Skip:
		return "SKIP"
	}
	return ""
}

// Testline is a single TAP-Testline.
type Testline struct {
	Ok          bool          // Whether the Testpoint executed ok
	Num         int           // The number of the test
	Description string        // A short description
	Directive   Directive     // Whether the test was skipped or is a todo
	Explanation string        // A short explanation why the test was skipped/is a todo
	Diagnostic  string        // A more detailed diagnostic message about the failed test
	Time        time.Duration // Time it took to test
	YAML        []byte        // The inline YAML-document, if given
	SubTests    []*Testline   // Sub-Tests
}

// Testsuite is the outcome of a Testsuite
type Testsuite struct {
	Ok      bool          // Whether the Testsuite as a whole succeeded
	Tests   []*Testline   // Description of all Testlines
	Plan    int           // Number of tests intended to run (-1 means no plan)
	Version int           // version number of TAP
	Time    time.Duration // Time it took to test
}

// Parser is a TAP parser.
type Parser struct {
	scanner      *bufio.Scanner
	lastNum      int
	suite        Testsuite
	startAt      time.Time
	lastExecTime time.Time
}

// NewParser creates a new Parser.
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
			Plan:    -1,
			Version: DefaultTAPVersion,
		},
	}, nil
}

// Next returns next Testline.
func (p *Parser) Next() (*Testline, error) {
	t, err := p.next("")
	if t != nil {
		p.suite.Tests = append(p.suite.Tests, t)
	}
	return t, err
}

func (p *Parser) next(indent string) (*Testline, error) {
	t := &Testline{}
	var err error

	for {
		line := p.scanner.Text()

		// ignore indent
		if !strings.HasPrefix(line, indent) {
			return nil, nil
		}
		line = line[len(indent):]

		// version
		if strings.HasPrefix(line, "TAP version ") {
			version, err := strconv.Atoi(line[len("TAP version "):])
			if err != nil {
				return nil, err
			}
			if version != 13 {
				return nil, ErrUnsupportedVersion
			}
			p.suite.Version = version
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
			p.suite.Plan = plan
			if !p.scanner.Scan() {
				return nil, io.EOF
			}
			continue
		}

		// test
		if strings.HasPrefix(line, "ok ") {
			t, err = p.parseTestLine(true, line[len("ok "):], indent)
			break
		}
		if strings.HasPrefix(line, "not ok ") {
			t, err = p.parseTestLine(false, line[len("not ok "):], indent)
			break
		}

		// subtest
		if strings.HasPrefix(line, "    # Subtest:") || strings.HasPrefix(line, "# Subtest:") {
			t, err = p.parseSubTestline(indent)
			break
		}

		// unknown line. skip it...

		if !p.scanner.Scan() {
			return nil, io.EOF
		}
	}

	return t, err
}

// Suite returns the Testsuite.
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

	if p.suite.Plan < 0 || len(p.suite.Tests) != p.suite.Plan {
		p.suite.Ok = false
		return &p.suite, nil
	}

	return &p.suite, nil
}

func (p *Parser) parseTestLine(ok bool, line string, indent string) (*Testline, error) {
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
	if startDescription := strings.IndexRune(line[index:], '-'); startDescription >= 0 {
		index += startDescription + 1
	}
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
		directive = TODO
		explanation = strings.TrimSpace(directiveStr[4:])
	}
	if len(directiveStr) > 4 && strings.EqualFold(directiveStr[0:4], "SKIP") {
		directive = Skip
		explanation = strings.TrimSpace(directiveStr[4:])
	}

	// parse diagnostics
	diagnostics := []string{}
	var yaml []byte
	for {
		if !p.scanner.Scan() {
			return &Testline{
				Ok:          ok,
				Num:         num,
				Description: description,
				Directive:   directive,
				Explanation: explanation,
				Diagnostic:  strings.Join(diagnostics, ""),
				Time:        duration,
				YAML:        yaml,
			}, io.EOF
		}

		text := p.scanner.Text()

		// ignore indent
		if !strings.HasPrefix(text, indent) {
			break
		}
		text = text[len(indent):]

		if p.suite.Version == 13 && strings.TrimSpace(text) == "---" {
			yaml = p.parseYAML()
		}
		if len(text) <= 1 || text[0] != '#' || strings.HasPrefix(text, "# Subtest:") {
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
		YAML:        yaml,
	}, nil
}

func (p *Parser) parseSubTestline(indent string) (*Testline, error) {
	// skip '# Subtest: foobar' line
	if !p.scanner.Scan() {
		return nil, io.EOF
	}

	// parse subtests
	subindent := indent + "    "
	subtests := []*Testline{}
	for {
		subtest, err := p.next(subindent)
		if subtest == nil {
			break
		}
		subtests = append(subtests, subtest)
		if err != nil {
			return nil, err
		}
	}

	// parse result of subtests
PARSE_TESTLINE:
	t, err := p.next(indent)
	if t == nil && err == nil {
		// invalid TAP format, ignore it
		p.scanner.Scan()
		goto PARSE_TESTLINE
	}
	if t != nil {
		t.SubTests = subtests
	}
	return t, err
}

func (p *Parser) parseYAML() []byte {
	yaml := []string{}
	for p.scanner.Scan() {
		text := p.scanner.Text()
		if strings.TrimSpace(text) == "..." {
			p.scanner.Scan()
			break
		}
		yaml = append(yaml, text, "\n")
	}
	return []byte(strings.Join(yaml, ""))
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
		str = append(str, " - ", t.Description)
	}

	if t.Directive != None {
		str = append(str, " # ", t.Directive.String())
		if t.Explanation != "" {
			str = append(str, " ", t.Explanation)
		}
	}

	return strings.Join(str, "")
}

// GoString returns the detail of the test result.
func (t *Testline) GoString() string {
	var buf bytes.Buffer
	t.dump(&buf, "")
	return buf.String()
}

func (t *Testline) dump(w io.Writer, indent string) error {
	if len(t.SubTests) > 0 {
		io.WriteString(w, indent)
		io.WriteString(w, "    # Subtest:")
		if t.Description != "" {
			io.WriteString(w, " ")
			io.WriteString(w, t.Description)
		}
		io.WriteString(w, "\n")
		subindent := indent + "    "
		for _, t := range t.SubTests {
			t.dump(w, subindent)
		}
		io.WriteString(w, subindent)
		io.WriteString(w, "1..")
		io.WriteString(w, strconv.Itoa(len(t.SubTests)))
		io.WriteString(w, "\n")
	}
	io.WriteString(w, indent)
	io.WriteString(w, t.String())
	io.WriteString(w, "\n")
	if t.Diagnostic != "" {
		diagnostics := strings.Split(t.Diagnostic, "\n")
		for _, l := range diagnostics[:len(diagnostics)-1] {
			io.WriteString(w, indent)
			io.WriteString(w, "# ")
			io.WriteString(w, l)
			io.WriteString(w, "\n")
		}
	}
	if len(t.YAML) > 0 {
		io.WriteString(w, indent)
		io.WriteString(w, "---\n")
		yaml := strings.Split(string(t.YAML), "\n")
		for _, l := range yaml[:len(yaml)-1] {
			io.WriteString(w, indent)
			io.WriteString(w, l)
			io.WriteString(w, "\n")
		}
		io.WriteString(w, indent)
		io.WriteString(w, "...\n")
	}
	return nil
}
