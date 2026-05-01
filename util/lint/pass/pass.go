package pass

import (
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

// FindContainingFile 在 Pass 中查找包含某个 ast 节点的文件。
func FindContainingFile(pass *analysis.Pass, n ast.Node) *ast.File {
	fPos := pass.Fset.File(n.Pos())
	for _, f := range pass.Files {
		if pass.Fset.File(f.Pos()) == fPos {
			return f
		}
	}
	panic(fmt.Errorf("cannot file file for %v", n))
}

// HasNolintComment 在传入节点的注释中包含 "nolint:<nolintName>" 时返回 true。
func HasNolintComment(pass *analysis.Pass, n ast.Node, nolintName string) bool {
	f := FindContainingFile(pass, n)
	relevant, containing := findNodesInBlock(f, n)
	cm := ast.NewCommentMap(pass.Fset, containing, f.Comments)
	// 检查相关的 ast 节点中是否包含目标注释。
	nolintComment := "nolint:" + nolintName
	for _, cn := range relevant {
		// Ident 节点会把该 ident 上的所有注释都收集到 containing 中，
		// 这可能数量过多。设想 ident 上的注释出现在 decl 块的其他位置，
		// 我们不希望它影响后续的转换。
		//
		// 我们希望将其过滤为 ident 上、且位于相关语句内部的所有注释。
		// 为此，我们拒绝那些 "//" 位置在最外层相关节点之外的 ident 注释。
		// 具体做法是检查注释相对于最外层相关节点的位置。
		// 这是可行的，因为一个 ident 单独时不会有注释，因为 ident 不是语句。
		_, isIdent := cn.(*ast.Ident)
		for _, cg := range cm[cn] {
			for _, c := range cg.List {
				if !strings.Contains(c.Text, nolintComment) {
					continue
				}
				outermost := relevant[len(relevant)-1]
				if isIdent && (cg.Pos() < outermost.Pos() || cg.End() > outermost.End()) {
					continue
				}
				return true
			}
		}
	}
	return false
}

// findNodesInBlock 查找出现在距离 n 最近的 block 或 decl 之下的所有表达式和语句。
// 其设计意图是：我们希望找出位于包含 ast 节点 n 的 block 或 decl 中的注释，以便
// 进行过滤。我们希望把注释筛选为所有与 n 关联的注释，或者从 n 到最近外围 block
// 或 decl 中的语句这一路径上任意表达式相关的注释。这是为了处理跨多行的表达式，
// 或者处理相关表达式出现在 if/for 的初始化子句中、注释位于上一行的情况。
//
// 例如，假设 n 是下面代码片段中方法 foo 上的 *ast.CallExpr：
//
//	func nonsense() bool {
//	    if v := (g.foo() + 1) > 2; !v {
//	        return true
//	    }
//	    return false
//	}
//
// 该函数会将一直到 `IfStmt` 的所有节点作为 relevant 返回，并将函数 nonsense 的
// `BlockStmt` 作为 containing 返回。
func findNodesInBlock(f *ast.File, n ast.Node) (relevant []ast.Node, containing ast.Node) {
	stack, _ := astutil.PathEnclosingInterval(f, n.Pos(), n.End())
	// 把 n 的所有子节点加入 relevant 节点集合。
	ast.Walk(funcVisitor(func(node ast.Node) {
		relevant = append(relevant, node)
	}), n)

	// 上面添加节点时父节点在前、子节点在后，这里将其反转。
	reverseNodes(relevant)

	// 向上添加父节点，直到外层的 block 或 declaration block，并找出该 containing 节点。
	containing = f // 最坏情况
	for _, n := range stack {
		switch n.(type) {
		case *ast.GenDecl, *ast.BlockStmt:
			containing = n
			return relevant, containing
		default:
			// 把 n 一直到外围 BlockStmt 或 GenDecl 之间的所有父节点都加入 relevant 集合。
			relevant = append(relevant, n)
		}
	}
	return relevant, containing
}

func reverseNodes(n []ast.Node) {
	for i := 0; i < len(n)/2; i++ {
		n[i], n[len(n)-i-1] = n[len(n)-i-1], n[i]
	}
}

type funcVisitor func(node ast.Node)

var _ ast.Visitor = (funcVisitor)(nil)

func (f funcVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node != nil {
		f(node)
	}
	return f
}
