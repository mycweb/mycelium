package lexer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLex(t *testing.T) {
	t.Parallel()
	type testCase struct {
		I string
		O []Token
	}
	mkCase := func(in string, toks ...Token) testCase {
		return testCase{in, toks}
	}
	tcs := []testCase{
		mkCase("", []Token{}...),
		mkCase("()", mkTok(LParen, 0), mkTok(RParen, 1)),
		mkCase("(   )", mkTok(LParen, 0), mkTok(RParen, 4)),
		mkCase("(( ) ) ", mkTok(LParen, 0), mkTok(LParen, 1), mkTok(RParen, 3), mkTok(RParen, 5)),

		mkCase("1234", mkInt(1234, 0)),
		mkCase("1 2 3 4", mkInt(1, 0), mkInt(2, 2), mkInt(3, 4), mkInt(4, 6)),
		mkCase("0xff", Token{Int, "0xff", Span{0, 4}}),
		mkCase("0o755", Token{Int, "0o755", Span{0, 5}}),

		mkCase(`"hello world"`, mkStr("hello world", 0)),
		mkCase(`"hello\n"`, mkStr("hello\n", 0)),

		mkCase("@abcd-_012345_ABCD", mkRef("abcd-_012345_ABCD", 0)),
		mkCase("abc123", mkSym("abc123", 0)),

		mkCase(`(symbol 117 "abc" )`,
			mkTok(LParen, 0), mkSym("symbol", 1), mkInt(117, 8), mkStr("abc", 12), mkTok(RParen, 18),
		),
		mkCase("1234 ;; this is a comment\n ()",
			mkInt(1234, 0), mkCOL(" this is a comment", 5), mkTok(LParen, 27), mkTok(RParen, 28),
		),
		mkCase("!hello", mkPrim("hello", 0)),
	}
	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Log(tc.I)
			l := NewLexer(strings.NewReader(tc.I))
			// collect all the tokens
			actual := []Token{}
			for range tc.O {
				tok, err := l.Next()
				require.NoError(t, err)
				require.False(t, tok.IsEOF())
				actual = append(actual, tok)
			}
			tok, err := l.Next()
			require.NoError(t, err)
			require.True(t, tok.IsEOF())

			require.Equal(t, tc.O, actual)
		})
	}
}

func mkInt(x int64, pos Pos) Token {
	text := fmt.Sprintf("%d", x)
	return Token{
		ty:   Int,
		text: text,
		span: Span{pos, pos + Pos(len(text))},
	}
}

func mkStr(x string, pos Pos) Token {
	text := fmt.Sprintf("%q", x)
	return Token{
		ty:   String,
		text: text,
		span: Span{pos, pos + Pos(len(text))},
	}
}

func mkRef(x string, pos Pos) Token {
	return Token{
		ty:   Ref,
		text: "@" + x,
		span: Span{pos, pos + Pos(len(x)) + 1},
	}
}

func mkPrim(x string, pos Pos) Token {
	return Token{
		ty:   Primitive,
		text: "!" + x,
		span: Span{pos, pos + Pos(len(x)) + 1},
	}
}

func mkSym(x string, pos Pos) Token {
	return Token{
		ty:   Symbol,
		text: x,
		span: Span{pos, pos + Pos(len(x))},
	}
}

// mkCOL makes a One-Line Comment
func mkCOL(x string, pos Pos) Token {
	text := fmt.Sprintf(";;%s", x)
	return Token{
		ty:   CommentOneLine,
		text: text,
		span: Span{pos, pos + Pos(len(text))},
	}
}
