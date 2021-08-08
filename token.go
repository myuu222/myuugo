package main

import (
	"strconv"
	"strings"
	"unicode"
)

// (先頭の識別子, 識別子を切り出して得られた残りの文字列)  を返す
func getIdentifier(s string) (string, string) {
	var res = ""
	for i, c := range s {
		if (i == 0 && unicode.IsDigit(c)) || !(isAlnum(c) || (c == '_')) {
			return res, s[i:]
		}
		res += string(c)
	}
	return res, ""
}

type TokenKind string

const (
	TokenNumber       TokenKind = "NUMBER"
	TokenIdentifier   TokenKind = "IDENTIFIER"
	TokenEof          TokenKind = "EOF"
	TokenReturn       TokenKind = "return"
	TokenIf           TokenKind = "if"
	TokenElse         TokenKind = "else"
	TokenFor          TokenKind = "for"
	TokenFunc         TokenKind = "func"
	TokenVar          TokenKind = "var"
	TokenPackage      TokenKind = "package"
	TokenEqual        TokenKind = "="
	TokenDoubleEqual  TokenKind = "=="
	TokenNotEqual     TokenKind = "!="
	TokenLessEqual    TokenKind = "<="
	TokenGreaterEqual TokenKind = ">="
	TokenLess         TokenKind = "<"
	TokenGreater      TokenKind = ">"
	TokenPlus         TokenKind = "+"
	TokenMinus        TokenKind = "-"
	TokenStar         TokenKind = "*"
	TokenSlash        TokenKind = "/"
	TokenLparen       TokenKind = "("
	TokenRparen       TokenKind = ")"
	TokenLbrace       TokenKind = "{"
	TokenRbrace       TokenKind = "}"
	TokenSemicolon    TokenKind = ";"
	TokenNewLine      TokenKind = "\n"
	TokenComma        TokenKind = ","
	TokenAmpersand    TokenKind = "&"
	TokenLSBrace      TokenKind = "["
	TokenRSBrace      TokenKind = "]"
)

type Token struct {
	kind TokenKind // トークンの型
	val  int       // kindがNumberの場合、その数値
	str  string    // トークン文字列
	rest string    // 自信を含めた残りすべてのトークン文字列
}

func (t Token) Test(kind TokenKind) bool {
	return t.kind == kind
}

func NewToken(kind TokenKind, str string, rest string) Token {
	return Token{kind: kind, str: str, rest: rest}
}

type Tokenizer struct {
	tokens []Token // inputをトークナイズした結果
	pos    int     // 現在着目しているトークンの添数
	input  string  // ユーザーからの入力プログラム
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{}
}

func (t *Tokenizer) Tokenize(input string) {
	t.tokens = []Token{}
	t.pos = 0
	t.input = input

	var symbols = []TokenKind{
		TokenDoubleEqual, TokenNotEqual, TokenGreaterEqual, TokenLessEqual,
		TokenPlus, TokenMinus, TokenStar, TokenSlash, TokenLparen, TokenRparen, TokenLess, TokenGreater, TokenSemicolon, TokenNewLine, TokenEqual, TokenLbrace, TokenRbrace, TokenComma, TokenAmpersand, TokenLSBrace, TokenRSBrace,
	}
	var keywords = []TokenKind{
		TokenPackage,
		TokenReturn,
		TokenFunc, TokenElse,
		TokenFor, TokenVar,
		TokenIf,
	}

	for input != "" {
		var isSymbol = false
		for _, symbol := range symbols {
			var strSymbol = string(symbol)
			if strings.HasPrefix(input, strSymbol) {
				isSymbol = true
				t.tokens = append(t.tokens, NewToken(symbol, strSymbol, input))
				input = input[len(symbol):]
				break
			}
		}
		if isSymbol {
			continue
		}

		var c = runeAt(input, 0)
		if isAlpha(c) || (c == '_') {
			// input から 識別子を取り出す
			var identifier, nextInput = getIdentifier(input)
			var isKeyword = false

			for _, keyword := range keywords {
				if keyword == TokenKind(identifier) {
					isKeyword = true
					t.tokens = append(t.tokens, NewToken(keyword, string(keyword), input))
					input = nextInput
					break
				}
			}
			if !isKeyword {
				var token = NewToken(TokenIdentifier, identifier, input)
				t.tokens = append(t.tokens, token)
				input = nextInput
			}
			continue
		}

		if unicode.IsSpace(c) {
			input = input[1:]
			continue
		}
		if unicode.IsDigit(c) {
			var token = NewToken(TokenNumber, "", input)
			token.val, input = strtoi(input)
			token.str = strconv.Itoa(token.val)
			t.tokens = append(t.tokens, token)
			continue
		}
		if c == '\'' {
			var token = NewToken(TokenNumber, input[:3], input)
			input = input[1:]
			var content = runeAt(input, 0)
			input = input[1:]
			var close = runeAt(input, 0)
			if close != '\'' {
				madden("文字リテラルの指定が不正です")
			}
			token.val = int(content)
			t.tokens = append(t.tokens, token)
			input = input[1:]
			continue
		}
		errorAt(input, "トークナイズできません")
	}
	t.tokens = append(t.tokens, NewToken(TokenEof, "", ""))
}

// 現在のトークンを返す
func (t *Tokenizer) Fetch() Token {
	return t.tokens[t.pos]
}

func (t *Tokenizer) Prefetch(n int) Token {
	return t.tokens[t.pos+n]
}

func (t *Tokenizer) Succ() {
	t.pos = t.pos + 1
}

func (t *Tokenizer) Test(kind TokenKind) bool {
	return t.Fetch().Test(kind)
}

// 次のトークンの種類が kind だった場合にはトークンを1つ読み進めて真を返す。
// それ以外の場合には偽を返す。
func (t *Tokenizer) Consume(kind TokenKind) bool {
	if t.Test(kind) {
		t.Succ()
		return true
	}
	return false
}

// 次のトークンが期待しているkindのときには、トークンを1つ読み進める。
// それ以外の場合にはエラーを報告する。
func (t *Tokenizer) Expect(kind TokenKind) {
	if !t.Consume(kind) {
		madden("'%s'ではありません", kind)
	}
}
