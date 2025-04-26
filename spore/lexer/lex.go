package lexer

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"myceliumweb.org/mycelium/internal/cadata"
)

const Base64Alphabet = cadata.Base64Alphabet

type stateFunc func() stateFunc

type Lexer struct {
	r io.RuneReader

	peeking   []rune
	err       error
	state     stateFunc
	bufOffset Pos
	buf       []rune
	output    chan Token
}

func NewLexer(r io.RuneReader) *Lexer {
	l := &Lexer{
		r: r,

		output: make(chan Token, 2),
	}
	l.state = l.lexInit
	return l
}

func (l *Lexer) Next() (Token, error) {
	for len(l.output) == 0 && l.err == nil {
		nextState := l.state()
		l.state = nextState
	}
	if l.err != nil {
		return Token{}, l.err
	}
	tok := <-l.output
	return tok, nil
}

// emit creates a token from the current buffer with type ty and emits it.
// emit clears the buffer
func (l *Lexer) emit(ty TokenType) {
	if ty == EOF {
		l.buf = append(l.buf[:0], eofRune)
	}
	tokSize := Pos(len(l.buf))
	l.output <- Token{
		ty: ty,
		span: Span{
			Begin: l.bufOffset,
			End:   l.bufOffset + tokSize,
		},
		text: string(l.buf),
	}
	l.bufOffset += tokSize
	l.buf = l.buf[:0]
}

// read consumes input
// if an error is encountered it sets l.err and returns eofRune
func (l *Lexer) read() rune {
	if len(l.peeking) > 0 {
		var r rune
		l.peeking, r = pop(l.peeking)
		l.buf = append(l.buf, r)
		return r
	}
	r, _, err := l.r.ReadRune()
	if err != nil {
		if err != io.EOF {
			l.err = err
			return eofRune
		} else {
			r = eofRune
		}
	}
	l.buf = append(l.buf, r)
	return r
}

// back puts r back into the input, ahead of everything.
// it can only be called once per call of read.
func (l *Lexer) back() {
	var r rune
	l.buf, r = pop(l.buf)
	l.peeking = append(l.peeking, r)
}

// peek returns the result of the next call to read without affecting the lexer's position.
func (l *Lexer) peek() rune {
	if len(l.peeking) == 0 {
		l.read()
		l.back()
	}
	return l.peeking[len(l.peeking)-1]
}

// stateInit is the initial state of the lexer
func (l *Lexer) lexInit() stateFunc {
	r := l.read()
	switch {
	case r == eofRune:
		return l.lexEnd
	case isWhitespace(r):
		l.back()
		return l.skipWhitespace
	case r == '(':
		l.emit(LParen)
	case r == ')':
		l.emit(RParen)
	case r == '[':
		l.emit(LBracket)
	case r == ']':
		l.emit(RBracket)
	case r == '{':
		l.emit(LBrace)
	case r == '}':
		l.emit(RBrace)
	case r == '\'':
		l.emit(SQuote)
	case r == '\n':
		l.emit(NewLine)
	case r == ':':
		l.emit(Colon)
	case r == ',':
		l.emit(Comma)
	// number
	case r == '+' || r == '-' || isDigit(r):
		l.back()
		return l.lexInt
	// string
	case r == '"':
		l.back()
		return l.lexString
	// ref
	case r == '@':
		l.back()
		return l.lexRef
	// comment
	case r == ';':
		if l.accept(";") {
			return l.lexComment()
		}
		l.emit(Illegal)
		return l.lexEnd
	case r == '%':
		l.back()
		return l.lexParam()
	case r == '!':
		return l.lexPrim()
	case isAlphanum(r):
		l.back()
		return l.lexSymbol
	default:
		l.emit(Illegal)
		return l.lexEnd
	}
	return l.lexInit
}

func (l *Lexer) lexSymbol() stateFunc {
	l.accum(isSymbol)
	if r := l.peek(); !isWhitespace(r) && !isOneOf(r, ")]}:,") && r != eofRune {
		return l.errorf("improperly terminated symbol %q", r)
	}
	l.emit(Symbol)
	return l.lexInit
}

func (l *Lexer) lexInt() stateFunc {
	l.accept("+-")

	digits := "0123456789"
	if l.accept("0") {
		// check for hex and octal
		if l.accept("x") {
			digits = "0123456789abcdefABCDEF"
		} else if l.accept("o") {
			digits = "01234567"
		}
	}
	digits += "_"
	l.acceptRun(digits)

	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun(digits)
	}
	l.emit(Int)
	return l.lexInit
}

func (l *Lexer) lexString() stateFunc {
	if !l.accept(`"`) {
		panic("not the beginning of a string")
	}
	for {
		r := l.read()
		if r == '\n' || r < 0 {
			return l.errorf("string literal not terminated")
		}
		if r == '"' {
			break
		}
		if r == '\\' {
			if !l.scanEscape('"') {
				return l.lexEnd
			}
		}
	}
	l.emit(String)
	return l.lexInit
}

func (l *Lexer) scanEscape(quote rune) bool {
	r := l.read()
	var n int
	var base, max uint32
	switch r {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		return true
	case 'x':
		r = l.read()
		n, base, max = 2, 16, 255
	case 'u':
		r = l.read()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		r = l.read()
		n, base, max = 8, 16, unicode.MaxRune
	default:
		l.errorf("unknown escape sequence")
		if r < 0 {
			l.errorf("escape sequence not terminated")
		}
		return false
	}

	var x uint32
	for n > 0 {
		d := uint32(digitVal(r))
		if d >= base {
			l.errorf("illegal character %#U in escape sequence", r)
			if r < 0 {
				l.errorf("escape sequence not terminated")
			}
			return false
		}
		x = x*base + d
		r = l.read()
		n--
	}

	if x > max || 0xD800 <= x && x < 0xE000 {
		l.errorf("escape sequence is invalid Unicode code point")
		return false
	}

	return true
}

func (l *Lexer) lexRef() stateFunc {
	if !l.accept("@") {
		panic("ref must start with @")
	}
	l.acceptRun(Base64Alphabet)
	if r := l.peek(); !isWhitespace(r) && r != ')' && r != eofRune {
		return l.errorf("improperly terminated symbol %q", r)
	}
	l.emit(Ref)
	return l.lexInit
}

func (l *Lexer) lexComment() stateFunc {
	l.accum(func(r rune) bool {
		switch r {
		case '\n', eofRune:
			return false
		default:
			return true
		}
	})
	l.emit(CommentOneLine)
	return l.lexInit
}

func (l *Lexer) lexParam() stateFunc {
	if !l.accept("%") {
		panic("param must start with %")
	}
	l.acceptRun("0123456789abcdef")
	l.emit(Param)
	return l.lexInit
}

func (l *Lexer) lexPrim() stateFunc {
	if l.accept("!") {
		panic("prim must start with !")
	}
	l.accum(isSymbol)
	if r := l.peek(); !isWhitespace(r) && !isOneOf(r, ")]}") && r != eofRune {
		return l.errorf("improperly terminated symbol %q", r)
	}
	l.emit(Primitive)
	return l.lexInit
}

// lexEnd is the terminal state of the lexer, indicating that it will only return EOF tokens.
func (l *Lexer) lexEnd() stateFunc {
	l.emit(EOF)
	return l.lexEnd
}

func (l *Lexer) accept(valid string) bool {
	if r := l.read(); strings.ContainsRune(valid, r) {
		return true
	} else {
		l.back()
		return false
	}
}

func (l *Lexer) acceptRun(valid string) {
	for l.accept(valid) {
	}
}

func (l *Lexer) ignore() {
	l.buf, _ = pop(l.buf)
	l.bufOffset++
}

func (l *Lexer) accum(fn func(rune) bool) {
	for {
		r := l.read()
		if !fn(r) {
			l.back()
			return
		}
	}
}

// skipWhitespace advances through the whitespace without emitting any tokens.
func (l *Lexer) skipWhitespace() stateFunc {
	for {
		r := l.read()
		if isWhitespace(r) {
			l.ignore()
		} else {
			l.back()
			return l.lexInit
		}
	}
}

func (l *Lexer) errorf(fstr string, args ...any) stateFunc {
	l.err = fmt.Errorf(fstr, args...)
	return l.lexEnd
}

func isWhitespace(ch rune) bool {
	return unicode.IsSpace(ch)
}
func isSymbol(ch rune) bool {
	return isAlphanum(ch) || isOneOf(ch, "<>./?!")
}
func isAlphanum(ch rune) bool {
	return isLetter(ch) || isDigit(ch)
}
func isLetter(ch rune) bool {
	return 'a' <= lower(ch) && lower(ch) <= 'z' || ch == '_' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}
func isDigit(ch rune) bool {
	return isDecimal(ch) || ch >= utf8.RuneSelf && unicode.IsDigit(ch)
}
func lower(ch rune) rune     { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }

func isOneOf(ch rune, xs string) bool {
	for _, x := range xs {
		if ch == x {
			return true
		}
	}
	return false
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= lower(ch) && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // larger than any legal digit val
}

func pop[E any, S ~[]E](s S) (S, E) {
	l := len(s)
	return s[:l-1], s[l-1]
}

const eofRune = -1
