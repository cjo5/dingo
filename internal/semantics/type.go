package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) checkTypes() {
	c.resetWalkState()
	c.checkModuleSet(true)
	c.checkModuleSet(false)
}

func (c *checker) checkModuleSet(signature bool) {
	c.set.ResetDeclColors()
	c.signature = signature
	for _, mod := range c.set.Modules {
		for _, decl := range mod.Decls {
			c.checkTopDecl(decl)
		}
	}
}

func (c *checker) checkDependencyGraph(decl ir.TopDecl) {
	graph := *decl.DependencyGraph()
	for dep, node := range graph {
		if !node.Weak {
			c.checkTopDecl(dep)
		}
	}
}

func (c *checker) checkTopDecl(decl ir.TopDecl) {
	if decl.Symbol() == nil {
		return
	}

	if decl.Color() != ir.WhiteColor {
		return
	}

	sym := decl.Symbol()
	defer setScope(setScope(c, sym.Parent))

	c.pushTopDecl(decl)
	decl.SetColor(ir.GrayColor)
	c.typeswitchDecl(decl)
	decl.SetColor(ir.BlackColor)
	c.popTopDecl()
}

func (c *checker) setWeakDependencies(decl ir.TopDecl) {
	graph := *decl.DependencyGraph()
	switch decl.(type) {
	case *ir.TypeTopDecl:
	case *ir.ValTopDecl:
	case *ir.FuncDecl:
		for k, v := range graph {
			sym := k.Symbol()
			if sym.ID == ir.FuncSymbol {
				v.Weak = true
			}
		}
	case *ir.StructDecl:
		for _, v := range graph {
			weakCount := 0
			for i := 0; i < len(v.Links); i++ {
				link := v.Links[i]
				if link.Sym == nil || link.Sym.T == nil {
					weakCount++
				} else if link.IsType {
					t := link.Sym.T
					if t.ID() == ir.TPointer || t.ID() == ir.TSlice || t.ID() == ir.TFunc {
						weakCount++
					}
				}
			}
			if weakCount == len(v.Links) {
				v.Weak = true
			}
		}
	default:
		panic(fmt.Sprintf("Unhandled top decl %T", decl))
	}
}

func (c *checker) typeswitchDecl(decl ir.Decl) {
	switch decl := decl.(type) {
	case *ir.TypeTopDecl:
		if !c.signature {
			return
		}
		c.setWeakDependencies(decl)
		c.checkDependencyGraph(decl)
		c.checkTypeDecl(decl.Sym, &decl.TypeDeclSpec)
	case *ir.TypeDecl:
		if decl.Sym != nil {
			c.checkTypeDecl(decl.Sym, &decl.TypeDeclSpec)
		}
	case *ir.ValTopDecl:
		if c.signature {
			return
		}
		c.setWeakDependencies(decl)
		c.checkDependencyGraph(decl)
		c.checkValDecl(decl.Sym, &decl.ValDeclSpec, true)
		if !ir.IsUntyped(decl.Sym.T) && decl.Sym.ID != ir.ConstSymbol {
			init := decl.Initializer
			if !checkCompileTimeConstant(init) {
				c.error(init.Pos(), "top-level initializer must be a compile-time constant")
			}
		}
	case *ir.ValDecl:
		if decl.Sym != nil {
			c.checkValDecl(decl.Sym, &decl.ValDeclSpec, decl.Init())
		}
	case *ir.FuncDecl:
		c.checkFuncDecl(decl)
	case *ir.StructDecl:
		c.checkStructDecl(decl)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) checkTypeDecl(sym *ir.Symbol, decl *ir.TypeDeclSpec) {
	decl.Type = c.checkTypeExpr(decl.Type, true, false)
	tdecl := decl.Type.Type()

	if ir.IsUntyped(tdecl) {
		sym.T = ir.TBuiltinUntyped
	} else {
		sym.T = tdecl
	}
}

func (c *checker) checkValDecl(sym *ir.Symbol, decl *ir.ValDeclSpec, defaultInit bool) {
	if decl.Decl.OneOf(token.Const, token.Val) {
		sym.Flags |= ir.SymFlagReadOnly
	}

	var tdecl ir.Type

	if decl.Type != nil {
		decl.Type = c.checkTypeExpr(decl.Type, true, true)
		tdecl = decl.Type.Type()

		if ir.IsUntyped(tdecl) {
			sym.T = tdecl
			return
		}
	}

	if decl.Initializer != nil {
		decl.Initializer = c.makeTypedExpr(decl.Initializer, tdecl)
		tinit := decl.Initializer.Type()

		if decl.Type == nil {
			if tdecl == nil {
				tdecl = tinit
			}
		} else {
			if ir.IsUntyped(tdecl) || ir.IsUntyped(tinit) {
				tdecl = ir.TBuiltinUntyped
			} else if !tdecl.Equals(tinit) {
				c.error(decl.Initializer.Pos(), "type mismatch %s and %s", tdecl, tinit)
				tdecl = ir.TBuiltinUntyped
			}
		}

		if decl.Decl.Is(token.Const) && !ir.IsUntyped(tdecl) {
			if !checkCompileTimeConstant(decl.Initializer) {
				c.error(decl.Initializer.Pos(), "const initializer must be a compile-time constant")
				tdecl = ir.TBuiltinUntyped
			}
		}
	} else if decl.Type == nil {
		c.error(decl.Name.Pos(), "missing type or initializer")
		tdecl = ir.TBuiltinUntyped
	} else if defaultInit {
		decl.Initializer = createDefaultLit(tdecl)
	}

	// Wait to set type until the final step in order to be able to detect cycles
	sym.T = tdecl

	if decl.Decl.Is(token.Const) && !ir.IsUntyped(tdecl) {
		c.constExprs[sym] = decl.Initializer
	}
}

func checkCompileTimeConstant(expr ir.Expr) bool {
	constant := true

	switch t := expr.(type) {
	case *ir.BasicLit:
	case *ir.ConstExpr:
	case *ir.StructLit:
		for _, arg := range t.Args {
			if !checkCompileTimeConstant(arg.Value) {
				return false
			}
		}
	case *ir.ArrayLit:
		for _, elem := range t.Initializers {
			if !checkCompileTimeConstant(elem) {
				return false
			}
		}
	case *ir.Ident:
		if t.Sym == nil || t.Sym.ID != ir.FuncSymbol {
			return false
		}
	default:
		constant = false
	}

	return constant
}

func (c *checker) checkFuncDecl(decl *ir.FuncDecl) {
	if c.signature {
		c.checkDependencyGraph(decl)

		c.checkIdent(decl.Name)

		defer setScope(setScope(c, decl.Scope))

		var params []ir.Field
		untyped := false

		for _, param := range decl.Params {
			c.typeswitchDecl(param)
			if param.Sym == nil || ir.IsUntyped(param.Sym.T) {
				untyped = true
			}

			if !untyped {
				params = append(params, ir.Field{Name: param.Sym.Name, T: param.Sym.T})
			}
		}

		decl.Return.Type = c.checkTypeExpr(decl.Return.Type, true, false)
		tret := decl.Return.Type.Type()

		if ir.IsUntyped(tret) {
			untyped = true
		}

		tfun := ir.TBuiltinUntyped

		if !untyped {
			cabi := (decl.Sym.ABI == ir.CABI)
			tfun = ir.NewFuncType(params, tret, cabi)
			if decl.Sym.T != nil && !checkTypes(decl.Sym.T, tfun) {
				c.error(decl.Name.Pos(), "redeclaration of '%s' (previously declared with a different signature at %s)",
					decl.Name.Literal, decl.Sym.DeclPos)
			}
		}

		if decl.Sym.T == nil {
			decl.Sym.T = tfun
		}

		return
	} else if decl.SignatureOnly() {
		return
	}

	c.checkDependencyGraph(decl)
	c.setWeakDependencies(decl)

	defer setScope(setScope(c, decl.Scope))
	c.typeswitchStmt(decl.Body)
}

func (c *checker) checkStructDecl(decl *ir.StructDecl) {
	if decl.Opaque && decl.Sym.IsDefined() {
		return
	}

	if c.signature {
		if decl.Sym.T == nil {
			decl.Sym.T = ir.NewStructType(decl.Sym, decl.Scope)
		}
		return
	}

	c.checkDependencyGraph(decl)

	untyped := false
	defer setScope(setScope(c, decl.Scope))

	for _, field := range decl.Fields {
		c.typeswitchDecl(field)
		if field.Sym == nil || ir.IsUntyped(field.Sym.T) {
			untyped = true
		}
	}

	c.setWeakDependencies(decl)

	if !untyped {
		var fields []ir.Field
		for _, field := range decl.Fields {
			fields = append(fields, ir.Field{Name: field.Sym.Name, T: field.Type.Type()})
		}
		tstruct := ir.ToBaseType(decl.Sym.T).(*ir.StructType)
		tstruct.SetBody(fields)
	}
}

func (c *checker) typeswitchStmt(stmt ir.Stmt) {
	switch stmt := stmt.(type) {
	case *ir.BlockStmt:
		defer setScope(setScope(c, stmt.Scope))
		stmtList(stmt.Stmts, c.typeswitchStmt)
	case *ir.DeclStmt:
		c.typeswitchDecl(stmt.D)
	case *ir.IfStmt:
		stmt.Cond = c.typeswitchExpr(stmt.Cond)
		if !checkTypes(stmt.Cond.Type(), ir.TBuiltinBool) {
			c.error(stmt.Cond.Pos(), "condition should have type %s (got %s)", ir.TBool, stmt.Cond.Type())
		}
		c.typeswitchStmt(stmt.Body)
		if stmt.Else != nil {
			c.typeswitchStmt(stmt.Else)
		}
	case *ir.ForStmt:
		defer setScope(setScope(c, stmt.Body.Scope))
		if stmt.Init != nil {
			c.typeswitchDecl(stmt.Init)
		}
		if stmt.Cond != nil {
			stmt.Cond = c.typeswitchExpr(stmt.Cond)
			if !checkTypes(stmt.Cond.Type(), ir.TBuiltinBool) {
				c.error(stmt.Cond.Pos(), "condition should have type %s (got %s)", ir.TBool, stmt.Cond.Type())
			}
		}
		if stmt.Inc != nil {
			c.typeswitchStmt(stmt.Inc)
		}
		c.loopCount++
		stmtList(stmt.Body.Stmts, c.typeswitchStmt)
		c.loopCount--
	case *ir.ReturnStmt:
		mismatch := false
		funDecl, _ := c.topDecl().(*ir.FuncDecl)
		retType := funDecl.Return.Type.Type()
		if ir.IsUntyped(retType) {
			return
		}
		exprType := ir.TBuiltinVoid
		if stmt.X == nil {
			if retType.ID() != ir.TVoid {
				mismatch = true
			}
		} else {
			stmt.X = c.makeTypedExpr(stmt.X, retType)

			if !checkTypes(stmt.X.Type(), retType) {
				exprType = stmt.X.Type()
				mismatch = true
			}
		}
		if mismatch {
			c.error(stmt.Pos(), "function has return type %s (got %s)", retType, exprType)
		}
	case *ir.DeferStmt:
		c.scope.Defer = true
		c.typeswitchStmt(stmt.S)
	case *ir.BranchStmt:
		if c.loopCount == 0 {
			c.error(stmt.Pos(), "'%s' can only be used in a loop", stmt.Tok)
		}
	case *ir.AssignStmt:
		stmt.Left = c.typeswitchExpr(stmt.Left)
		left := stmt.Left
		if ir.IsUntyped(left.Type()) {
			// Do nothing
		} else if !left.Lvalue() {
			c.error(left.Pos(), "expression is not an lvalue")
		} else if left.ReadOnly() {
			c.error(left.Pos(), "expression is read-only")
		} else {
			stmt.Right = c.makeTypedExpr(stmt.Right, left.Type())
			right := stmt.Right
			if !checkTypes(left.Type(), right.Type()) {
				c.errorNode(stmt, "type mismatch %s and %s", left.Type(), right.Type())
			}
			if stmt.Assign != token.Assign {
				if !ir.IsNumericType(left.Type()) {
					c.errorNode(left, "type %s is not numeric", left.Type())
				}
			}
		}
	case *ir.ExprStmt:
		stmt.X = c.makeTypedExpr(stmt.X, nil)
		if stmt.X.Type().ID() == ir.TUntyped {
			common.Assert(c.errors.IsError(), "expr at %s is untyped and no error was reported", stmt.X.Pos())
		}
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", stmt))
	}
}

func (c *checker) typeswitchExpr(expr ir.Expr) ir.Expr {
	switch expr := expr.(type) {
	case *ir.PointerTypeExpr:
		return c.checkPointerTypeExpr(expr)
	case *ir.ArrayTypeExpr:
		return c.checkArrayTypeExpr(expr)
	case *ir.FuncTypeExpr:
		return c.checkFuncTypeExpr(expr)
	case *ir.Ident:
		return c.checkIdent(expr)
	case *ir.BasicLit:
		return c.checkBasicLit(expr)
	case *ir.StructLit:
		return c.checkStructLit(expr)
	case *ir.ArrayLit:
		return c.checkArrayLit(expr)
	case *ir.BinaryExpr:
		return c.checkBinaryExpr(expr)
	case *ir.UnaryExpr:
		return c.checkUnaryExpr(expr)
	case *ir.DotExpr:
		return c.checkDotExpr(expr)
	case *ir.IndexExpr:
		return c.checkIndexExpr(expr)
	case *ir.SliceExpr:
		return c.checkSliceExpr(expr)
	case *ir.FuncCall:
		return c.checkFuncCall(expr)
	case *ir.CastExpr:
		return c.checkCastExpr(expr)
	case *ir.LenExpr:
		return c.checkLenExpr(expr)
	case *ir.SizeExpr:
		return c.checkSizeExpr(expr)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", expr))
	}
}

func (c *checker) checkTypeExpr(expr ir.Expr, root bool, checkVoid bool) ir.Expr {
	prevMode := c.exprMode
	c.exprMode = exprModeType
	expr = c.typeswitchExpr(expr)
	c.exprMode = prevMode
	if root {
		if expr.Type().ID() != ir.TVoid || checkVoid {
			if ir.IsIncompleteType(expr.Type(), nil) {
				c.errorNode(expr, "incomplete type %s", expr.Type())
				expr.SetType(ir.TBuiltinUntyped)
			}
		}
	}
	return expr
}

func (c *checker) checkPointerTypeExpr(expr *ir.PointerTypeExpr) ir.Expr {
	expr.X = c.checkTypeExpr(expr.X, false, false)

	ro := expr.Decl.Is(token.Val)
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		expr.T = ir.TBuiltinUntyped
	} else if tslice, ok := tx.(*ir.SliceType); ok {
		if !tslice.Ptr {
			tslice.Ptr = true
			tslice.ReadOnly = ro
			expr.T = tslice
		} else {
			expr.T = ir.NewPointerType(tx, ro)
		}
	} else {
		expr.T = ir.NewPointerType(tx, ro)
	}

	return expr
}

func (c *checker) checkArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	arraySize := 0

	if expr.Size != nil {
		prevMode := c.exprMode
		c.exprMode = exprModeNone
		expr.Size = c.makeTypedExpr(expr.Size, ir.TBuiltinInt32)
		c.exprMode = prevMode

		err := false

		size := expr.Size
		if ir.IsUntyped(size.Type()) {
			err = true
		} else if size.Type().ID() != ir.TInt32 {
			c.error(expr.Size.Pos(), "array size must be of type %s (got %s)", ir.TInt32, size.Type())
			err = true
		} else if lit, ok := size.(*ir.BasicLit); !ok {
			c.error(expr.Size.Pos(), "array size is not a constant expression")
			err = true
		} else if lit.NegatigeInteger() {
			c.error(expr.Size.Pos(), "array size cannot be negative")
			err = true
		} else if lit.Zero() {
			c.error(expr.Size.Pos(), "array size cannot be zero")
			err = true
		} else {
			arraySize = int(lit.AsU64())
		}

		if err {
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	expr.X = c.checkTypeExpr(expr.X, false, false)
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		expr.T = ir.TBuiltinUntyped
	} else if arraySize != 0 {
		expr.T = ir.NewArrayType(tx, arraySize)
	} else {
		expr.T = ir.NewSliceType(tx, true, false)
	}

	return expr
}

func (c *checker) checkFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	var params []ir.Field
	untyped := false
	for i, param := range expr.Params {
		expr.Params[i].Type = c.checkTypeExpr(param.Type, true, true)
		if !untyped {
			tparam := expr.Params[i].Type.Type()
			params = append(params, ir.Field{Name: param.Name.Literal, T: tparam})
			if ir.IsUntyped(tparam) {
				untyped = true
			}
		}
	}

	expr.Return.Type = c.checkTypeExpr(expr.Return.Type, true, false)
	if ir.IsUntyped(expr.Return.Type.Type()) {
		untyped = true
	}

	if !untyped {
		cabi := false
		if expr.ABI != nil {
			if expr.ABI.Literal == ir.CABI {
				cabi = true
			} else if !ir.IsValidABI(expr.ABI.Literal) {
				c.error(expr.ABI.Pos(), "unknown abi '%s'", expr.ABI.Literal)
			}
		}
		expr.T = ir.NewFuncType(params, expr.Return.Type.Type(), cabi)
	} else {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}
