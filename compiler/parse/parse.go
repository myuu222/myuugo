package parse

import (
	"os"
	"strings"

	"github.com/myuu222/myuugo/compiler/lang"
	"github.com/myuu222/myuugo/compiler/util"
)

var tokenizer *Tokenizer
var userInput string
var filename string

// トークナイザ拡張

// 文の終端記号であるトークンを1つ読み飛ばす。
// 戻り値は実際に読み飛ばしたかどうか。
func skipEndOfLine() bool {
	return tokenizer.Consume(TokenSemicolon) || tokenizer.Consume(TokenNewLine)
}

func endOfLine() string {
	token := tokenizer.Fetch()
	if !token.Test(TokenSemicolon) && !token.Test(TokenNewLine) {
		BadToken(token, "This is not end of line symbol.")
	}
	if tokenizer.Consume(TokenSemicolon) {
		return string(TokenSemicolon)
	}
	return string(TokenNewLine)
}

func numberLiteral() int {
	token := tokenizer.Fetch()
	if !token.Test(TokenNumber) {
		BadToken(token, "This is not number literal.")
	}
	tokenizer.Succ()
	return token.val
}

func boolLiteral() int {
	token := tokenizer.Fetch()
	if !token.Test(TokenBool) {
		BadToken(token, "This is not bool literal.")
	}
	tokenizer.Succ()
	return token.val
}

func stringLiteral() string {
	token := tokenizer.Fetch()
	if !token.Test(TokenString) {
		BadToken(token, "This is not string literal.")
	}
	tokenizer.Succ()
	return token.str
}

func identifier() string {
	token := tokenizer.Fetch()
	if !token.Test(TokenIdentifier) {
		BadToken(token, "This is not identifier.")
	}
	tokenizer.Expect(TokenIdentifier)
	return token.str
}

func isType() bool {
	if tokenizer.Test(TokenStar) || tokenizer.Test(TokenLSBrace) {
		return true
	}
	if !tokenizer.Test(TokenIdentifier) {
		return false
	}

	ident := tokenizer.Fetch().str
	if ident == "int" || ident == "rune" || ident == "bool" || ident == "string" || ident == "struct" {
		return true
	}
	_, ok := Env.program.FindType(ident)
	return ok
}

func type_() lang.Type {
	if tokenizer.Consume(TokenStar) {
		ty := type_()
		return lang.NewPointerType(&ty)
	}
	if tokenizer.Consume(TokenLSBrace) {
		if tokenizer.Consume(TokenRSBrace) {
			// スライス
			ty := type_()
			return lang.NewSliceType(ty)
		}
		var arraySize = numberLiteral()
		tokenizer.Expect(TokenRSBrace)
		ty := type_()
		return lang.NewArrayType(ty, arraySize)
	}

	ident := identifier()
	if ident == "int" {
		return lang.NewType(lang.TypeInt)
	}
	if ident == "rune" {
		return lang.NewType(lang.TypeRune)
	}
	if ident == "bool" {
		return lang.NewType(lang.TypeBool)
	}
	if ident == "string" {
		return lang.NewType(lang.TypeString)
	}
	if ident == "struct" {
		tokenizer.Expect(TokenLbrace)
		tokenizer.Expect(TokenNewLine)
		names, types := []string{}, []lang.Type{}
		for !tokenizer.Consume(TokenRbrace) {
			name := identifier()
			ty := type_()
			tokenizer.Expect(TokenNewLine)

			names = append(names, name)
			types = append(types, ty)
		}
		return lang.NewStructType(names, types)
	}
	ty, _ := Env.program.FindType(ident)
	return ty
}

var Env *Environment

func stepIn() {
	Env = Env.Fork()
}

func stepInFunction(name string) {
	Env = Env.Fork()
	Env.FunctionName = name
}

func stepOut() {
	Env = Env.parent
}

var source *Source

func forceTrailingSlash(path string) string {
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path
}

// path直下にgo.modがあれば読み込んで、module名を返す。
// なければ空文字列を返す。
func readModuleNameIfExists(path string) string {
	path = forceTrailingSlash(path)
	_, err := os.Stat(path + "go.mod")
	if err != nil {
		return ""
	}
	content := util.ReadFile(path + "go.mod")
	firstLine := strings.Split(content, "\n")[0]
	return strings.Split(firstLine, " ")[0]
}

var sourcePrefix string

func parseProgram(srcPrefix string, path string) *Program {
	sourcePrefix = srcPrefix

	goFilePaths := util.EnumerateGoFilePaths(path)
	Env = NewEnvironment()

	for _, p := range goFilePaths {
		stepIn()

		source = NewSource(p)
		tokenizer = NewTokenizer()
		tokenizer.Tokenize(p)

		for skipEndOfLine() {
		}
		source.Code = append(source.Code, packageStmt())
		endOfLine()

		for {
			for skipEndOfLine() {
			}
			if tokenizer.Test(TokenImport) {
				source.Code = append(source.Code, importStmt())
			} else {
				break
			}
		}
		source.Code = append(source.Code, topLevelStmtList().Children...)
		Env.program.Sources = append(Env.program.Sources, source)
		stepOut()
	}

	return Env.program
}

func includes(slice []string, e string) bool {
	for _, s := range slice {
		if s == e {
			return true
		}
	}
	return false
}

func findNextPackageName(programs []*Program) (string, bool) {
	var imported = []string{}
	for _, prog := range programs {
		for _, s := range prog.Sources {
			imported = append(imported, s.Packages...)
		}
	}

	for _, i := range imported {
		var parsed = false
		for _, p := range programs {
			if p.Name == i {
				parsed = true
				break
			}
		}
		if !parsed {
			return i, true
		}
	}
	return "", false
}

// Create an AST for all Go files directly under `path`.
func Parse(path string) []*Program {
	srcPrefix := readModuleNameIfExists(path)
	programs := []*Program{parseProgram(srcPrefix, path)}

	libraryPackageNames := []string{"os", "fmt", "strconv"}

	for {
		nextPackageName, ok := findNextPackageName(programs)
		if !ok {
			break
		}
		// 標準ライブラリだった場合

		if includes(libraryPackageNames, nextPackageName) {
			programs = append(programs, parseProgram("", "./library/"+nextPackageName))
			continue
		}
		// TODO: 自作パッケージだった場合
		break
	}
	return programs
}

func importStmt() *Node {
	tokenizer.Expect(TokenImport)
	packages := []string{}

	if tokenizer.Consume(TokenLparen) {
		// グループ化
		tokenizer.Expect(TokenNewLine)

		for !tokenizer.Consume(TokenRparen) {
			pkg := strings.Trim(stringLiteral(), "\"")
			packages = append(packages, pkg)
			tokenizer.Expect(TokenNewLine)

			source.AddPackage(pkg)
		}
		return NewImportStmtNode(packages)
	}
	packages = append(packages, strings.Trim(stringLiteral(), "\""))
	source.AddPackage(packages[0])
	return NewImportStmtNode(packages)
}

func packageStmt() *Node {
	var n = NewLeafNode(NodePackageStmt)

	tokenizer.Expect(TokenPackage)
	if sourcePrefix == "" {
		n.Label = identifier()
	} else {
		n.Label = sourcePrefix + "/" + identifier()
	}
	Env.program.Name = n.Label

	return n
}

func localStmtList() *Node {
	var stmts = make([]*Node, 0)
	var endLineRequired = false

	for !(tokenizer.Test(TokenRbrace)) {
		if endLineRequired {
			BadToken(tokenizer.Fetch(), "文の区切り文字が必要です")
		}
		if skipEndOfLine() {
			continue
		}
		stmts = append(stmts, localStmt())

		endLineRequired = true
		if skipEndOfLine() {
			endLineRequired = false
		}
	}
	var node = NewNode(NodeStmtList, stmts)
	node.Children = stmts
	return node
}

func topLevelStmtList() *Node {
	var stmts = make([]*Node, 0)
	var endLineRequired = false

	for !tokenizer.Test(TokenEof) && !(tokenizer.Test(TokenRbrace)) {
		if endLineRequired {
			BadToken(tokenizer.Fetch(), "文の区切り文字が必要です")
		}
		if skipEndOfLine() {
			continue
		}
		stmts = append(stmts, topLevelStmt())

		endLineRequired = true
		if skipEndOfLine() {
			endLineRequired = false
		}
	}
	var node = NewNode(NodeStmtList, stmts)
	node.Children = stmts
	return node
}

func typeStmt() *Node {
	tokenizer.Expect(TokenType)
	typeName := identifier()
	entityType := type_()
	definedType := lang.NewUserDefinedType(typeName, entityType)

	Env.program.RegisterType(definedType)

	return NewNode(NodeTypeStmt, []*Node{})
}

func topLevelStmt() *Node {
	// 関数定義
	if tokenizer.Test(TokenFunc) {
		return funcDefinition()
	}
	// var文
	if tokenizer.Test(TokenVar) {
		return topLevelVarStmt()
	}
	// typ文
	if tokenizer.Test(TokenType) {
		return typeStmt()
	}

	// 許可されていないもの
	if tokenizer.Test(TokenIf) {
		BadToken(tokenizer.Fetch(), "if文はトップレベルでは使用できません")
	}
	if tokenizer.Test(TokenFor) {
		BadToken(tokenizer.Fetch(), "for文はトップレベルでは使用できません")
	}
	if tokenizer.Test(TokenReturn) {
		BadToken(tokenizer.Fetch(), "return文はトップレベルでは使用できません")
	}
	BadToken(tokenizer.Fetch(), "トップレベルの文として許可されていません")
	return nil // 到達しない
}

func simpleStmt() *Node {
	if tokenizer.Test(TokenNewLine) || tokenizer.Test(TokenSemicolon) {
		return nil
	}

	var pos = 0
	var nxtToken = tokenizer.Prefetch(pos)
	for !nxtToken.Test(TokenNewLine) && !nxtToken.Test(TokenSemicolon) {
		if tokenizer.Prefetch(pos).Test(TokenEqual) {
			// 代入文としてパース
			var n = exprList()
			tokenizer.Expect(TokenEqual)
			return NewBinaryNode(NodeAssign, n, exprList())
		}
		if tokenizer.Prefetch(pos).Test(TokenColonEqual) {
			// 短絡変数宣言としてパース
			var n = localVarList()
			tokenizer.Expect(TokenColonEqual)
			return NewBinaryNode(NodeShortVarDeclStmt, n, exprList())
		}
		pos += 1
		nxtToken = tokenizer.Prefetch(pos)
	}
	return NewNode(NodeExprStmt, []*Node{expr()})
}

func localStmt() *Node {
	// if文
	if tokenizer.Test(TokenIf) {
		return metaIfStmt()
	}
	// for文
	if tokenizer.Test(TokenFor) {
		return forStmt()
	}
	// var文
	if tokenizer.Test(TokenVar) {
		return localVarStmt()
	}
	if tokenizer.Consume(TokenReturn) {
		if tokenizer.Test(TokenNewLine) || tokenizer.Test(TokenSemicolon) {
			// 空のreturn文
			return NewUnaryOperationNode(NodeReturn, nil)
		}
		return NewUnaryOperationNode(NodeReturn, exprList())
	}
	return simpleStmt()
}

// トップレベル変数は初期化式は与えないことにする
func topLevelVarStmt() *Node {
	tokenizer.Expect(TokenVar)
	var v = topLevelVariableDeclaration()
	v.Variable.Type = type_()
	return NewNode(NodeTopLevelVarStmt, []*Node{v})
}

func localVarStmt() *Node {
	tokenizer.Expect(TokenVar)
	var v = localVariableDeclaration()

	if !isType() {
		// 型が明示されていないときは初期化が必須
		tokenizer.Expect(TokenEqual)
		return NewBinaryNode(NodeLocalVarStmt, v, expr())
	}

	ty := type_()
	v.Variable.Type = ty
	if tokenizer.Consume(TokenEqual) {
		return NewBinaryNode(NodeLocalVarStmt, v, expr())
	}
	return NewNode(NodeLocalVarStmt, []*Node{v})
}

func funcDefinition() *Node {
	tokenizer.Expect(TokenFunc)
	ident := identifier()

	stepInFunction(ident)
	var fn = lang.NewFunction(Env.FunctionName, []lang.Type{}, lang.NewUndefinedType())
	Env.program.RegisterFunction(fn)

	var parameters = make([]*Node, 0)

	tokenizer.Expect(TokenLparen)
	for !tokenizer.Consume(TokenRparen) {
		if len(parameters) > 0 {
			tokenizer.Expect(TokenComma)
		}
		lvarNode := localVariableDeclaration()
		parameters = append(parameters, lvarNode)
		lvarNode.Variable.Type = type_()
		fn.ParameterTypes = append(fn.ParameterTypes, lvarNode.Variable.Type)
	}

	fn.ReturnValueType = lang.NewType(lang.TypeVoid)
	if tokenizer.Consume(TokenLparen) { // 多値
		var types = []lang.Type{type_()}
		for tokenizer.Consume(TokenComma) {
			types = append(types, type_())
		}
		tokenizer.Expect(TokenRparen)
		fn.ReturnValueType = lang.NewMultipleType(types)
	} else if isType() {
		fn.ReturnValueType = type_()
	}

	var node *Node

	if tokenizer.Consume(TokenLbrace) {
		var functionName = ident
		var body = localStmtList()

		tokenizer.Expect(TokenRbrace)
		node = NewFunctionDefNode(functionName, parameters, body)
		fn.IsDefined = true
	} else {
		// 関数宣言
		node = NewLeafNode(NodeStatementFunctionDeclaration)
	}
	stepOut()
	return node
}

// range は未対応
func forStmt() *Node {
	stepIn()
	tokenizer.Expect(TokenFor)
	// 初期化, ループ条件, 更新式, 繰り返す文

	if tokenizer.Consume(TokenLbrace) {
		// 無限ループ
		var body = localStmtList()
		tokenizer.Expect(TokenRbrace)
		stepOut()
		return NewForNode(nil, nil, nil, body)
	}

	var st = tokenizer.Fetch()
	var s = simpleStmt()
	if tokenizer.Consume(TokenLbrace) {
		// while文
		if s.Kind != NodeExprStmt {
			BadToken(st, "for文の条件に式以外が書かれています")
		}
		var cond = s.Children[0] // expr
		var body = localStmtList()
		tokenizer.Expect(TokenRbrace)
		stepOut()
		return NewForNode(nil, cond, nil, body)
	}

	// 通常のfor文
	var init = s
	tokenizer.Expect(TokenSemicolon)
	var cond = expr()
	tokenizer.Expect(TokenSemicolon)
	var update = simpleStmt()

	tokenizer.Expect(TokenLbrace)
	var body = localStmtList()
	tokenizer.Expect(TokenRbrace)
	stepOut()
	return NewForNode(init, cond, update, body)
}

func metaIfStmt() *Node {
	token := tokenizer.Fetch()
	if !token.Test(TokenIf) {
		BadToken(token, "'"+string(TokenIf)+"'ではありません")
	}

	var ifNode = ifStmt()
	if tokenizer.Test(TokenElse) {
		var elseNode = elseStmt()
		return NewMetaIfNode(ifNode, elseNode)
	}
	return NewMetaIfNode(ifNode, nil)
}

func ifStmt() *Node {
	stepIn()

	tokenizer.Expect(TokenIf)
	var cond = expr()
	tokenizer.Expect(TokenLbrace)
	var body = localStmtList()
	tokenizer.Expect(TokenRbrace)

	stepOut()
	return NewIfNode(cond, body)
}

func elseStmt() *Node {
	tokenizer.Expect(TokenElse)

	if tokenizer.Consume(TokenLbrace) {
		stepIn()
		var body = localStmtList()
		tokenizer.Expect(TokenRbrace)
		stepOut()
		return NewElseNode(body)
	}
	return metaIfStmt()
}

func localVarList() *Node {
	var lvars = []*Node{localVariableDeclaration()}
	for tokenizer.Consume(TokenComma) {
		lvars = append(lvars, localVariableDeclaration())
	}
	return NewNode(NodeLocalVarList, lvars)
}

func exprList() *Node {
	var exprs = []*Node{expr()}

	for tokenizer.Consume(TokenComma) {
		exprs = append(exprs, expr())
	}
	return NewNode(NodeExprList, exprs)
}

func expr() *Node {
	return logicalOr()
}

func logicalOr() *Node {
	var n = logicalAnd()
	for {
		if tokenizer.Consume(TokenDoubleVerticalLine) {
			n = NewBinaryOperationNode(NodeLogicalOr, n, logicalAnd())
		} else {
			return n
		}
	}
}

func logicalAnd() *Node {
	var n = equality()
	for {
		if tokenizer.Consume(TokenDoubleAmpersand) {
			n = NewBinaryOperationNode(NodeLogicalAnd, n, equality())
		} else {
			return n
		}
	}
}

func equality() *Node {
	var n = relational()
	for {
		if tokenizer.Consume(TokenDoubleEqual) {
			n = NewBinaryOperationNode(NodeEql, n, relational())
		} else if tokenizer.Consume(TokenNotEqual) {
			n = NewBinaryOperationNode(NodeNotEql, n, relational())
		} else {
			return n
		}
	}
}

func relational() *Node {
	var n = add()
	for {
		if tokenizer.Consume(TokenLess) {
			n = NewBinaryOperationNode(NodeLess, n, add())
		} else if tokenizer.Consume(TokenLessEqual) {
			n = NewBinaryOperationNode(NodeLessEql, n, add())
		} else if tokenizer.Consume(TokenGreater) {
			n = NewBinaryOperationNode(NodeGreater, n, add())
		} else if tokenizer.Consume(TokenGreaterEqual) {
			n = NewBinaryOperationNode(NodeGreaterEql, n, add())
		} else {
			return n
		}
	}
}

func add() *Node {
	var n = mul()
	for {
		if tokenizer.Consume(TokenPlus) {
			n = NewBinaryOperationNode(NodeAdd, n, mul())
		} else if tokenizer.Consume(TokenMinus) {
			n = NewBinaryOperationNode(NodeSub, n, mul())
		} else {
			return n
		}
	}
}

func mul() *Node {
	var n = unary()
	for {
		if tokenizer.Consume(TokenStar) {
			n = NewBinaryOperationNode(NodeMul, n, unary())
		} else if tokenizer.Consume(TokenSlash) {
			n = NewBinaryOperationNode(NodeDiv, n, unary())
		} else if tokenizer.Consume(TokenPercent) {
			n = NewBinaryOperationNode(NodeMod, n, unary())
		} else {
			return n
		}
	}
}

func unary() *Node {
	if tokenizer.Consume(TokenPlus) {
		return primary()
	}
	if tokenizer.Consume(TokenMinus) {
		return NewBinaryOperationNode(NodeSub, NewNodeNum(0), primary())
	}
	if tokenizer.Consume(TokenStar) {
		return NewUnaryOperationNode(NodeDeref, unary())
	}
	if tokenizer.Consume(TokenAmpersand) {
		return NewUnaryOperationNode(NodeAddr, unary())
	}
	if tokenizer.Consume(TokenBang) {
		return NewUnaryOperationNode(NodeNot, unary())
	}
	return primary()
}

func structTypeLiteral() *Node {
	token := tokenizer.Fetch()
	ident := identifier()
	ty, ok := Env.program.FindType(ident)
	if !ok {
		BadToken(token, "未定義の型のリテラルです")
	}
	names, values := []string{}, []*Node{}
	tokenizer.Expect(TokenLbrace)
	for !tokenizer.Consume(TokenRbrace) {
		if len(names) > 0 {
			tokenizer.Expect(TokenComma)
		}
		names = append(names, identifier())
		tokenizer.Expect(TokenColon)
		values = append(values, expr())
	}
	return NewStructLiteral(ty, names, values)
}

func primary() *Node {
	// 次のトークンが "(" なら、"(" expr ")" のはず
	if tokenizer.Consume(TokenLparen) {
		var n = expr()
		tokenizer.Expect(TokenRparen)
		return n
	}
	if tokenizer.Test(TokenNumber) {
		return NewNodeNum(numberLiteral())
	}
	if tokenizer.Test(TokenBool) {
		return NewNodeBool(boolLiteral())
	}
	if tokenizer.Test(TokenString) {
		var n = NewLeafNode(NodeString)
		n.Str = Env.program.AddStringLiteral(tokenizer.Fetch().str)
		tokenizer.Succ()
		return n
	}

	if tokenizer.Test(TokenLSBrace) {
		ty := type_()

		if ty.Kind == lang.TypeSlice {
			elements := []*Node{}
			tokenizer.Expect(TokenLbrace)
			for !tokenizer.Consume(TokenRbrace) {
				if len(elements) > 0 {
					tokenizer.Expect(TokenComma)
				}
				elements = append(elements, expr())
			}
			return NewSliceLiteral(ty, elements)
		}
		panic("未実装の型のリテラルです")
	}

	var tok = tokenizer.Fetch()
	ty, ok := Env.program.FindType(tok.str)
	// struct型のリテラル
	if ok {
		names, values := []string{}, []*Node{}
		type_()
		tokenizer.Expect(TokenLbrace)
		for !tokenizer.Consume(TokenRbrace) {
			if len(names) > 0 {
				tokenizer.Expect(TokenComma)
			}
			names = append(names, identifier())
			tokenizer.Expect(TokenColon)
			values = append(values, expr())
		}
		return NewStructLiteral(ty, names, values)
	}

	var pkgName = ""
	var name = tokenizer.Fetch().str
	pkgName, ok = source.FindPackage(name)

	if ok {
		identifier()
		tokenizer.Expect(TokenDot)
	}

	var n *Node = named()
	for {
		if tokenizer.Consume(TokenLSBrace) {
			n = NewIndexNode(n, expr())
			tokenizer.Expect(TokenRSBrace)
			continue
		}
		if tokenizer.Consume(TokenDot) {
			// メソッド呼び出しは一旦無視
			n = NewDotNode(n, identifier())
			continue
		}
		break
	}
	if ok {
		n.In = pkgName
		n = NewNode(NodePackageDot, []*Node{n})
		n.Label = pkgName
	}
	return n
}

func named() *Node {
	if tokenizer.Prefetch(1).Test(TokenLparen) {
		// append関数の呼び出し
		if tokenizer.Fetch().str == "append" {
			tokenizer.Expect(TokenIdentifier)
			tokenizer.Expect(TokenLparen)
			var arg1 = expr()
			tokenizer.Expect(TokenComma)
			var arg2 = expr()
			tokenizer.Expect(TokenRparen)
			return NewAppendCallNode(arg1, arg2)
		}
		// string関数の呼び出し
		if tokenizer.Fetch().str == "string" {
			tokenizer.Expect(TokenIdentifier)
			tokenizer.Expect(TokenLparen)
			var arg = expr()
			tokenizer.Expect(TokenRparen)
			return NewStringCallNode(arg)
		}
		// rune関数の呼び出し
		if tokenizer.Fetch().str == "rune" {
			tokenizer.Expect(TokenIdentifier)
			tokenizer.Expect(TokenLparen)
			var arg = expr()
			tokenizer.Expect(TokenRparen)
			return NewRuneCallNode(arg)
		}
		// len関数の呼び出し
		if tokenizer.Fetch().str == "len" {
			tokenizer.Expect(TokenIdentifier)
			tokenizer.Expect(TokenLparen)
			var arg = expr()
			tokenizer.Expect(TokenRparen)
			return NewLenCallNode(arg)
		}

		// 関数呼び出し
		var functionName = identifier()
		tokenizer.Expect(TokenLparen)
		var arguments = []*Node{}
		for !tokenizer.Consume(TokenRparen) {
			if len(arguments) > 0 {
				tokenizer.Expect(TokenComma)
			}
			arguments = append(arguments, expr())
		}
		return NewFunctionCallNode(functionName, arguments)
	}
	return variableRef()
}

func variableRef() *Node {
	ident := identifier()
	var v = Env.FindVar(ident)
	if v != nil && v.Kind == lang.VariableLocal {
		var node = NewLeafNode(NodeLocalVariable)
		node.Variable = v
		return node
	}
	var node = NewLeafNode(NodeTopLevelVariable)
	node.Label = ident
	return node
}

func localVariableDeclaration() *Node {
	var token = tokenizer.Fetch()
	var ident = identifier()
	var node = NewLeafNode(NodeLocalVariable)
	lvar := Env.FindLocalVar(ident)
	if lvar != nil {
		BadToken(token, "すでに定義済みの変数です")
	}
	node.Variable = Env.AddLocalVar(lang.NewUndefinedType(), ident)
	return node
}

func topLevelVariableDeclaration() *Node {
	var token = tokenizer.Fetch()
	var ident = identifier()
	var node = NewLeafNode(NodeTopLevelVariable)
	tvar := Env.program.FindTopLevelVariable(ident)
	if tvar != nil {
		BadToken(token, "すでに定義済みの変数です")
	}
	node.Variable = Env.program.AddTopLevelVariable(lang.NewUndefinedType(), ident)
	node.Label = ident
	return node
}
