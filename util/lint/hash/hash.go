package hash

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// Doc 是该 pass 的文档说明。
const Doc = `check for correct use of hash.Hash`

// Analyzer 定义了该 pass。
var Analyzer = &analysis.Analyzer{
	Name:     "hash",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// hashChecker 用于确保 hash.Hash 接口不被误用。一种常见的错误是认为 Sum 函数会
// 返回其输入数据的哈希值，例如：
//
//	hashedBytes := sha256.New().Sum(inputBytes)
//
// 实际上，Sum 的参数并不是要被哈希的字节，而是一个用作输出的切片，方便调用方
// 避免一次额外的内存分配。在上述示例中，hashedBytes 并不是 inputBytes 的
// SHA-256 哈希值，而是 inputBytes 与空字符串哈希值的拼接结果。
//
// hash.Hash 接口的正确用法如下：
//
//	h := sha256.New()
//	h.Write(inputBytes)
//	hashedBytes := h.Sum(nil)
//
//	h := sha256.New()
//	h.Write(inputBytes)
//	var hashedBytes [sha256.Size]byte
//	h.Sum(hashedBytes[:0])
//
// 为区分正确和错误的用法，hashChecker 采用了一个简单的启发式规则：当对 Sum 的
// 调用同时满足 a) 参数非 nil，b) 返回值被使用 时，就将其标记出来。
//
// hash.Hash 接口在 Go 2 中可能会得到改进，参见 golang/go#21070。
func run(pass *analysis.Pass) (any, error) {
	selectorIsHash := func(s *ast.SelectorExpr) bool {
		tv, ok := pass.TypesInfo.Types[s.X]
		if !ok {
			return false
		}
		named, ok := tv.Type.(*types.Named)
		if !ok {
			return false
		}
		if named.Obj().Type().String() != "hash.Hash" {
			return false
		}
		return true
	}

	stack := make([]ast.Node, 0, 32)
	forAllFiles(pass.Files, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1] // 出栈
			return true
		}
		stack = append(stack, n) // 入栈

		// 查找对 hash.Hash.Sum 的调用。
		selExpr, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selExpr.Sel.Name != "Sum" {
			return true
		}
		if !selectorIsHash(selExpr) {
			return true
		}
		callExpr, ok := stack[len(stack)-2].(*ast.CallExpr)
		if !ok {
			return true
		}
		if len(callExpr.Args) != 1 {
			return true
		}
		// 此时我们已找到一个对 hash.Hash.Sum 的有效调用。

		// 参数是否为 nil？
		var nilArg bool
		if id, ok := callExpr.Args[0].(*ast.Ident); ok && id.Name == "nil" {
			nilArg = true
		}

		// 返回值是否未被使用？
		var retUnused bool
	Switch:
		switch t := stack[len(stack)-3].(type) {
		case *ast.AssignStmt:
			for i := range t.Rhs {
				if t.Rhs[i] == stack[len(stack)-2] {
					if id, ok := t.Lhs[i].(*ast.Ident); ok && id.Name == "_" {
						// 赋值给空白标识符不算使用返回值。
						retUnused = true
					}
					break Switch
				}
			}
			panic("unreachable")
		case *ast.ExprStmt:
			// 表达式语句意味着返回值未被使用。
			retUnused = true
		default:
		}

		if !nilArg && !retUnused {
			pass.Reportf(callExpr.Pos(), "probable misuse of hash.Hash.Sum: "+
				"provide parameter or use return value, but not both")
		}
		return true
	})

	return nil, nil
}

func forAllFiles(files []*ast.File, fn func(node ast.Node) bool) {
	for _, f := range files {
		ast.Inspect(f, fn)
	}
}
