package lexer

import "fmt"

type TokenType int

const (
	// Special tokens
	Illegal TokenType = iota
	EOF

	// Identifiers and basic type literals
	// (these tokens stand for classes of literals)
	Symbol    // main
	Int       // 12345
	Char      // 'a'
	String    // "abc"
	Ref       // @abcdefgh123455
	Param     // %0 %1 %2
	Primitive // !make

	LParen   // (
	RParen   // )
	LBracket // [
	RBracket // ]
	LBrace   // {
	RBrace   // }
	Colon    // :
	Comma    // ,

	NewLine
	// CommentOneLine starts a single line comment
	CommentOneLine
	// CommentBegin starts a comment
	CommentBegin
	// CommentEnd ends a comment
	CommentEnd
	SQuote
)

type Token struct {
	ty   TokenType
	text string
	span Span
}

func (tok Token) Type() TokenType { return tok.ty }

func (tok Token) Text() string {
	return tok.text
}

func (tok Token) Slice(src []byte) []byte {
	return src[tok.span.Begin:tok.span.End]
}

func (tok Token) String() string {
	switch tok.ty {
	case EOF:
		return "EOF"
	}
	return fmt.Sprintf("%q", tok.text)
}

func (tok Token) Span() Span {
	return tok.span
}

func (tok Token) IsEOF() bool {
	return tok.Type() == EOF
}

func mkTok(ty TokenType, beg Pos) Token {
	text := map[TokenType]string{
		LParen:   "(",
		RParen:   ")",
		LBracket: "[",
		RBracket: "]",
		LBrace:   "{",
		RBrace:   "}",
	}[ty]
	return Token{
		ty:   ty,
		text: text,
		span: Span{
			beg,
			beg + Pos(len(text)),
		},
	}
}

// Pos is a position withing the input
type Pos uint32

// Span is a region of the input
type Span struct {
	Begin Pos
	End   Pos
}
