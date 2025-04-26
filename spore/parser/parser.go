package parser

import (
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/lexer"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/ringbuf"
)

type (
	Token = lexer.Token
	Pos   = lexer.Pos
	Node  = ast.Node
)

type Span struct {
	Bound    lexer.Span
	Children []Span
}

type Parser struct {
	lex   *lexer.Lexer
	inBuf ringbuf.RingBuf[Token]

	outBuf ringbuf.RingBuf[Node]
}

func NewParser(r io.RuneReader) *Parser {
	return &Parser{
		lex:    lexer.NewLexer(r),
		inBuf:  ringbuf.New[Token](1),
		outBuf: ringbuf.New[Node](2),
	}
}

func (p *Parser) ParseAST() (Span, Node, error) {
	span, node, err := p.parseOneExpr()
	if err != nil {
		return Span{}, nil, err
	}
	if node == nil {
		return Span{}, nil, nil
	}
	if err := p.fill(1); err != nil {
		return Span{}, nil, err
	}
	if p.inBuf.Len() == 0 || p.inBuf.At(0).Type() != lexer.Colon {
		return span, node, nil
	}
	p.inBuf.PopFront() // get rid of the colon
	span2, node2, err := p.parseOneExpr()
	if err != nil {
		return Span{}, nil, err
	}
	return combineSpans(span, span2), ast.Row{Key: node, Value: node2}, nil
}

func (p *Parser) parseOneExpr() (Span, Node, error) {
	tok, err := p.next()
	if err != nil {
		return Span{}, nil, err
	}
	switch tok.Type() {
	case lexer.EOF:
		return Span{}, nil, nil
	case lexer.Int:
		return p.parseInt(tok)
	case lexer.String:
		return p.parseString(tok)
	case lexer.Symbol:
		return p.parseSymbol(tok)
	case lexer.Primitive:
		return p.parseOp(tok)
	case lexer.Ref:
		return p.parseRef(tok)
	case lexer.Param:
		return p.parseParam(tok)
	case lexer.CommentOneLine:
		return p.parseComment(tok)
	case lexer.LParen:
		p.back(tok)
		return p.ParseSExpr()
	case lexer.LBracket:
		p.back(tok)
		return p.parseArray()
	case lexer.LBrace:
		p.back(tok)
		return p.parseTupleOrTable()
	case lexer.SQuote:
		span, node, err := p.ParseAST()
		if err != nil {
			return Span{}, nil, err
		}
		span.Bound.Begin--
		return span, ast.Quote{X: node}, nil
	case lexer.Colon:
		return p.parseOneExpr()
	default:
		return Span{}, nil, fmt.Errorf("unexpected token %v", tok)
	}
}

func (p *Parser) ParseSExpr() (Span, Node, error) {
	span, nodes, err := p.parseCompound(lexer.LParen, lexer.RParen, false)
	if err != nil {
		return Span{}, nil, err
	}
	return span, ast.SExpr(nodes), nil
}

func (p *Parser) parseArray() (Span, Node, error) {
	span, nodes, err := p.parseCompound(lexer.LBracket, lexer.RBracket, true)
	if err != nil {
		return Span{}, nil, err
	}
	return span, ast.Array(nodes), nil
}

func (p *Parser) parseTupleOrTable() (Span, Node, error) {
	span, nodes, err := p.parseCompound(lexer.LBrace, lexer.RBrace, true)
	if err != nil {
		return Span{}, nil, err
	}
	var rows []ast.Row
	for _, node := range nodes {
		if row, ok := node.(ast.Row); ok {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return span, ast.Tuple(nodes), nil
	} else if len(rows) == len(nodes) {
		return span, ast.Table(rows), nil
	} else {
		return span, nil, fmt.Errorf("table contains-non rows / tuple contains rows")
	}
}

func (p *Parser) parseCompound(beg, end lexer.TokenType, allowCommas bool) (Span, []Node, error) {
	tok, err := p.next()
	if err != nil {
		return Span{}, nil, err
	}
	if tok.Type() != beg {
		panic(tok)
	}
	span := Span{Bound: tok.Span()}
	exprs := []Node{}
	for {
		tok, err := p.next()
		if err != nil {
			return Span{}, nil, err
		}
		if tok.Type() == lexer.EOF || tok.Type() == end {
			span.Bound.End = tok.Span().End
			break
		} else if allowCommas && tok.Type() == lexer.Comma {
			continue
		} else {
			p.back(tok)
		}
		span2, subExpr, err := p.ParseAST()
		if err != nil {
			return Span{}, nil, err
		}
		span.Children = append(span.Children, span2)
		exprs = append(exprs, subExpr)
	}
	return span, exprs, nil
}

func (p *Parser) parseSymbol(tok Token) (Span, Node, error) {
	sym := tok.Text()
	return Span{Bound: tok.Span()}, ast.Symbol(sym), nil
}

func (p *Parser) parseInt(tok Token) (Span, Node, error) {
	if tok.Type() != lexer.Int {
		return Span{}, nil, fmt.Errorf("cannot parser number from token %v", tok)
	}
	n := new(big.Int)
	if err := n.UnmarshalText([]byte(tok.Text())); err != nil {
		return Span{}, nil, err
	}
	return Span{Bound: tok.Span()}, ast.NewBigInt(n), nil
}

func (p *Parser) parseString(tok Token) (Span, Node, error) {
	if tok.Type() != lexer.String {
		return Span{}, nil, fmt.Errorf("cannot parse string from non-string token %v", tok)
	}
	s := tok.Text()
	s = s[1 : len(s)-1]
	// TODO: more escape sequences
	r := strings.NewReplacer(
		"\\n", "\n",
		"\\r", "\r",
	)
	s = r.Replace(s)
	return Span{Bound: tok.Span()}, ast.String(s), nil
}

func (p *Parser) parseOp(tok Token) (Span, Node, error) {
	return Span{Bound: tok.Span()}, ast.Op(tok.Text()[1:]), nil
}

func (p *Parser) parseRef(tok Token) (Span, Node, error) {
	var ref [32]byte
	enc := base64.NewEncoding(cadata.Base64Alphabet)
	if _, err := enc.Decode(ref[:], []byte(tok.Text())[1:]); err != nil {
		return Span{}, nil, err
	}
	return Span{Bound: tok.Span()}, ast.Ref(ref), nil
}

func (p *Parser) parseParam(tok Token) (Span, Node, error) {
	n, err := strconv.ParseUint(tok.Text()[1:], 10, 0)
	if err != nil {
		return Span{}, nil, err
	}
	return Span{Bound: tok.Span()}, ast.Param(n), nil
}

func (p *Parser) parseComment(tok Token) (Span, Node, error) {
	return Span{Bound: tok.Span()}, ast.Comment(tok.Text()[2:]), nil
}

func (p *Parser) ParseSymbol() (*string, error) {
	tok, err := p.next()
	if err != nil {
		return nil, err
	}
	if tok.Type() != lexer.Symbol {
		return nil, fmt.Errorf("cannot parse symbol from non-symbol token %v", tok)
	}
	s := tok.Text()
	return &s, nil
}

func (p *Parser) fill(n int) error {
	for p.inBuf.Len() < n {
		tok, err := p.lex.Next()
		if err != nil {
			return err
		}
		p.inBuf.PushBack(tok)
		if tok.Type() == lexer.EOF {
			break
		}
	}
	return nil
}

func (p *Parser) next() (ret Token, _ error) {
	if err := p.fill(1); err != nil {
		return Token{}, err
	}
	return p.inBuf.PopFront(), nil
}

func (p *Parser) back(tok Token) {
	p.inBuf.PushFront(tok)
}

func combineSpans(spans ...Span) (ret Span) {
	ret.Children = spans
	ret.Bound.Begin = spans[0].Bound.Begin
	ret.Bound.End = spans[len(spans)-1].Bound.End
	return ret
}

func ReadAll(p *Parser) (rootSpan Span, ret []Node, _ error) {
	for {
		span, e, err := p.ParseAST()
		if err != nil {
			return span, nil, err
		}
		if e == nil {
			break
		}
		rootSpan.Children = append(rootSpan.Children, span)
		ret = append(ret, e)
	}
	return rootSpan, ret, nil
}
