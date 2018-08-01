package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) checkTypes() {
	c.step = 0
	for _, objList := range c.objectMatrix {
		c.objectList = objList
		for _, obj := range objList.objects {
			c.checkDgObject(obj)
		}
	}
	c.step++
	for decl := range c.incomplete {
		c.checkIncompleteDgObject(decl)
	}
}

func (c *checker) checkIncompleteDgObject(obj *dgObject) {
	if obj.color != whiteColor {
		return
	}
	obj.color = grayColor
	for dep := range obj.deps {
		if obj.incomplete {
			c.checkIncompleteDgObject(dep)
		}
	}
	c.objectList = c.objectMatrix[obj.CUID()]
	c.checkDgObject(obj)
	obj.color = blackColor
}

func (c *checker) checkDgObject(obj *dgObject) {
	c.object = obj
	switch decl := obj.d.(type) {
	case *ir.ImportDecl:
		c.checkImportDecl(decl)
		c.object.checked = true
	case *ir.TypeDecl:
		c.checkTypeDecl(decl)
		c.object.checked = true
	case *ir.ValDecl:
		c.checkValDecl(decl)
		c.object.checked = true
		if !isUntyped(decl.Sym.T) {
			init := decl.Initializer
			if !decl.Sym.IsConst() {
				c.error(init.Pos(), "top-level initializer must be a compile-time constant")
				decl.Sym.T = ir.TBuiltinInvalid
			}
		}
	case *ir.FuncDecl:
		c.checkFuncDecl(decl)
	case *ir.StructDecl:
		c.object.checked = true
		c.checkStructDecl(decl)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) checkLocalDecl(decl ir.Decl) {
	switch decl := decl.(type) {
	case *ir.ImportDecl:
		if c.step == 0 {
			c.insertImportSymbols(decl, c.object.CUID(), c.object.modFQN(), false, false)
		}
		if decl.Sym != nil {
			c.checkImportDecl(decl)
		}
	case *ir.TypeDecl:
		if c.step == 0 {
			c.setLocalTypeDeclSymbol(decl, c.object.CUID(), c.object.modFQN())
		}
		if decl.Sym != nil {
			c.checkTypeDecl(decl)
		}
		if c.step == 0 {
			decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
		}
	case *ir.ValDecl:
		if c.step == 0 {
			c.setLocalValDeclSymbol(decl, c.object.CUID(), c.object.modFQN())
		}
		if decl.Sym != nil {
			c.checkValDecl(decl)
		}
		if c.step == 0 {
			decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
		}
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) checkImportDecl(decl *ir.ImportDecl) {
	if isUntyped(decl.Sym.T) {
		return
	}
	for _, item := range decl.Items {
		itemSym := item.Name.Sym
		if itemSym == nil || !isUnresolvedType(itemSym.T) {
			continue
		}
		tmod := decl.Sym.T.(*ir.ModuleType)
		importedSym := tmod.Scope.Lookup(itemSym.Name)
		titem := ir.TBuiltinInvalid
		if importedSym == nil {
			c.error(item.Name.Pos(), "undeclared identifier '%s' in module '%s'", item.Name.Literal, decl.Sym.ModFQN)
		} else if importedSym.IsBuiltin() {
			c.error(item.Name.Pos(), "builtin '%s' cannot be imported", importedSym.Name)
		} else {
			visibilityOK := true
			if decl.Decl == token.Import {
				if !importedSym.Public {
					c.error(item.Name.Pos(), "private '%s' cannot be imported", importedSym.Name)
					visibilityOK = false
				}
			} else {
				if !importedSym.Public && itemSym.Public {
					c.error(item.Name.Pos(), "private '%s' cannot be re-exported as public", importedSym.Name)
					visibilityOK = false
				}
			}
			if visibilityOK {
				c.tryAddDep(importedSym, itemSym.Pos)
				titem = importedSym.T
				if !isUnresolvedType(titem) {
					itemSym.Kind = importedSym.Kind
					itemSym.Key = importedSym.Key
					itemSym.Flags |= (importedSym.Flags & (ir.SymFlagReadOnly | ir.SymFlagConst))
				}
			}
		}
		itemSym.T = titem
	}
}

func (c *checker) checkTypeDecl(decl *ir.TypeDecl) {
	if !isUnresolvedType(decl.Sym.T) {
		return
	}
	decl.Type = c.checkRootTypeExpr(decl.Type, false)
	decl.Sym.T = decl.Type.Type()
}

func (c *checker) checkValDecl(decl *ir.ValDecl) {
	if !isUnresolvedType(decl.Sym.T) {
		return
	}
	if decl.Type != nil {
		decl.Type = c.checkRootTypeExpr(decl.Type, true)
	}
	if decl.Initializer != nil {
		decl.Initializer = c.checkExpr(decl.Initializer)
	}
	if tpunt := puntExprs(decl.Type, decl.Initializer); tpunt != nil {
		decl.Sym.T = tpunt
		return
	}
	tres := ir.TBuiltinInvalid
	if decl.Initializer != nil {
		var tdecl ir.Type
		if decl.Type != nil {
			tdecl = decl.Type.Type()
			decl.Initializer = c.finalizeExpr(decl.Initializer, tdecl)
		} else {
			decl.Initializer = c.finalizeExpr(decl.Initializer, nil)
			tdecl = decl.Initializer.Type()
		}
		tinit := decl.Initializer.Type()
		if tdecl.Equals(tinit) {
			tres = tdecl
		} else {
			c.error(decl.Initializer.Pos(), "type mismatch %s and %s", tdecl, tinit)
		}
	} else {
		tres = decl.Type.Type()
	}
	if !isUntyped(tres) {
		if decl.Initializer == nil {
			decl.Initializer = ir.NewDefaultInit(tres)
		}
		if decl.Decl.Is(token.Val) {
			decl.Sym.Flags |= ir.SymFlagReadOnly
		}
		if checkCompileTimeConstant(decl.Initializer) {
			decl.Sym.Flags |= ir.SymFlagConst
		}
	}
	decl.Sym.T = tres
}

func checkCompileTimeConstant(expr ir.Expr) bool {
	constant := true
	switch t := expr.(type) {
	case *ir.BasicLit:
	case *ir.ConstExpr:
	case *ir.DefaultInit:
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
		if t.Sym == nil || t.Sym.Kind != ir.FuncSymbol {
			return false
		}
	default:
		constant = false
	}
	return constant
}

func (c *checker) checkFuncDecl(decl *ir.FuncDecl) {
	defer c.setScope(c.setScope(decl.Scope))
	if isUnresolvedType(decl.Sym.T) {
		var tpunt ir.Type
		for _, param := range decl.Params {
			if param.Sym != nil {
				c.checkValDecl(param)
				tpunt = untyped(param.Sym.T, tpunt)
			} else {
				tpunt = ir.TBuiltinInvalid
			}
		}
		decl.Return.Type = c.checkRootTypeExpr(decl.Return.Type, false)
		tret := decl.Return.Type.Type()
		tpunt = untyped(tret, tpunt)
		if tpunt != nil {
			decl.Sym.T = tpunt
		} else {
			var params []ir.Field
			for _, param := range decl.Params {
				params = append(params, ir.Field{Name: param.Sym.Name, T: param.Type.Type()})
			}
			cabi := (decl.Sym.ABI == ir.CABI)
			tfun := ir.NewFuncType(params, tret, cabi)
			if isTypeMismatch(decl.Sym.T, tfun) {
				c.error(decl.Name.Pos(), "redeclaration of '%s' (different declaration is at %s)", decl.Name.Literal, decl.Sym.Pos)
				decl.Sym.T = ir.TBuiltinInvalid
			} else {
				decl.Sym.T = tfun
			}
		}
		c.object.checked = true
	}
	if !decl.SignatureOnly() {
		c.checkStmt(decl.Body)
	}
}

func (c *checker) checkStructDecl(decl *ir.StructDecl) {
	if decl.Opaque && decl.Sym.IsDefined() {
		return
	}
	tstruct := ir.ToBaseType(decl.Sym.T).(*ir.StructType)
	if tstruct.TypedBody {
		return
	}
	var tpunt ir.Type
	for _, field := range decl.Fields {
		if field.Sym != nil {
			c.checkValDecl(field)
			tpunt = untyped(field.Sym.T, tpunt)
		} else {
			tpunt = ir.TBuiltinInvalid
		}
	}
	typedBody := true
	if tpunt != nil {
		if tpunt.Kind() == ir.TInvalid {
			decl.Sym.T = tpunt
			return
		}
		typedBody = false
	}
	var fields []ir.Field
	for _, field := range decl.Fields {
		if field.Sym != nil {
			fields = append(fields, ir.Field{Name: field.Sym.Name, T: field.Type.Type()})
		}
	}
	tstruct.SetBody(fields, typedBody)
}

func (c *checker) checkStmt(stmt ir.Stmt) {
	switch stmt := stmt.(type) {
	case *ir.BlockStmt:
		if c.step == 0 {
			c.openScope(ir.LocalScope)
			stmt.Scope = c.scope
			c.closeScope()
		}
		prevScope := c.setScope(stmt.Scope)
		stmtList(stmt.Stmts, c.checkStmt)
		c.setScope(prevScope)
	case *ir.DeclStmt:
		c.checkLocalDecl(stmt.D)
	case *ir.IfStmt:
		if isUnresolvedExpr(stmt.Cond) {
			stmt.Cond = c.checkExpr(stmt.Cond)
			if isTypeMismatch(stmt.Cond.Type(), ir.TBuiltinBool) {
				c.error(stmt.Cond.Pos(), "condition expects type %s (got %s)", ir.TBool, stmt.Cond.Type())
				stmt.Cond.SetType(ir.TBuiltinInvalid)
			}
		}
		c.checkStmt(stmt.Body)
		if stmt.Else != nil {
			c.checkStmt(stmt.Else)
		}
	case *ir.ForStmt:
		if c.step == 0 {
			c.openScope(ir.LocalScope)
			stmt.Body.Scope = c.scope
			c.closeScope()
		}
		prevScope := c.setScope(stmt.Body.Scope)
		if stmt.Init != nil {
			c.checkStmt(stmt.Init)
		}
		if stmt.Cond != nil {
			if isUnresolvedExpr(stmt.Cond) {
				stmt.Cond = c.checkExpr(stmt.Cond)
				if isTypeMismatch(stmt.Cond.Type(), ir.TBuiltinBool) {
					c.error(stmt.Cond.Pos(), "condition expects type %s (got %s)", ir.TBool, stmt.Cond.Type())
					stmt.Cond.SetType(ir.TBuiltinInvalid)
				}
			}
		}
		if stmt.Inc != nil {
			c.checkStmt(stmt.Inc)
		}
		c.loop++
		stmtList(stmt.Body.Stmts, c.checkStmt)
		c.loop--
		c.setScope(prevScope)
	case *ir.ReturnStmt:
		if stmt.X != nil && isUnresolvedExpr(stmt.X) {
			stmt.X = c.checkExpr(stmt.X)
			if isUntypedExpr(stmt.X) {
				return
			}
		}
		fun := c.object.d.(*ir.FuncDecl)
		tret := fun.Return.Type.Type()
		if isUntyped(tret) {
			return
		}
		if stmt.X == nil {
			stmt.X = ir.NewIdent1(token.Placeholder)
			stmt.X.SetType(ir.TBuiltinVoid)
		} else {
			stmt.X = c.finalizeExpr(stmt.X, tret)
		}
		texpr := stmt.X.Type()
		if isTypeMismatch(texpr, tret) {
			c.error(stmt.Pos(), "function expects return type %s (got %s)", tret, texpr)
			stmt.X.SetType(ir.TBuiltinInvalid)
		}
	case *ir.DeferStmt:
		c.scope.Defer = true
		c.checkStmt(stmt.S)
	case *ir.BranchStmt:
		if c.step == 0 && c.loop == 0 {
			c.error(stmt.Pos(), "'%s' can only be used in a loop", stmt.Tok)
		}
	case *ir.AssignStmt:
		if isUnresolvedExprs(stmt.Left, stmt.Right) {
			stmt.Left = c.checkExpr(stmt.Left)
			stmt.Right = c.checkExpr(stmt.Right)
			if puntExprs(stmt.Left, stmt.Right) != nil {
				return
			}
			left := stmt.Left
			err := false
			if !left.Lvalue() {
				err = true
				c.error(left.Pos(), "expression is not an lvalue")
			} else if left.ReadOnly() {
				err = true
				c.error(left.Pos(), "expression is read-only")
			}
			if stmt.Assign != token.Assign {
				if !ir.IsNumericType(left.Type()) {
					err = true
					c.nodeError(left, "type %s is not numeric", left.Type())
				}
			}
			if !err {
				stmt.Right = c.finalizeExpr(stmt.Right, left.Type())
				right := stmt.Right
				if isTypeMismatch(left.Type(), right.Type()) {
					c.nodeError(stmt, "type mismatch %s and %s", left.Type(), right.Type())
				}
			}
		}
	case *ir.ExprStmt:
		if isUnresolvedExpr(stmt.X) {
			stmt.X = c.checkExpr(stmt.X)
			stmt.X = c.finalizeExpr(stmt.X, nil)
		}
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", stmt))
	}
}

func (c *checker) checkExpr2(expr ir.Expr, mode int) ir.Expr {
	prevMode := c.setMode(mode)
	expr = c.checkExpr(expr)
	c.mode = prevMode
	return expr
}

func (c *checker) checkExpr(expr ir.Expr) ir.Expr {
	if !isUnresolvedExpr(expr) {
		return expr
	}
	switch expr := expr.(type) {
	case *ir.PointerTypeExpr:
		return c.checkPointerTypeExpr(expr)
	case *ir.SliceTypeExpr:
		return c.checkSliceTypeExpr(expr)
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
	case *ir.ConstExpr:
		return expr
	default:
		panic(fmt.Sprintf("Unhandled expr %T at %s", expr, expr.Pos()))
	}
}

func (c *checker) finalizeExpr(expr ir.Expr, target ir.Type) ir.Expr {
	if isUntypedExpr(expr) || (target != nil && isUntyped(target)) {
		return expr
	}

	checkIncomplete := false
	texpr := expr.Type()

	switch expr := expr.(type) {
	case *ir.SliceExpr:
		checkIncomplete = true
	case *ir.UnaryExpr:
		if expr.Op.OneOf(token.Addr, token.Deref) {
			checkIncomplete = true
		}
	}

	if checkIncomplete && isIncompleteType(texpr, nil) {
		c.nodeError(expr, "expression has incomplete type %s", texpr)
		expr.SetType(ir.TBuiltinInvalid)
		return expr
	}

	return ensureCompatibleType(expr, target)
}

func (c *checker) checkRootTypeExpr(expr ir.Expr, checkVoid bool) ir.Expr {
	expr = c.checkExpr2(expr, modeType)
	texpr := expr.Type()
	if !isUntyped(texpr) {
		if texpr.Kind() != ir.TVoid || checkVoid {
			if isIncompleteType(texpr, nil) {
				c.nodeError(expr, "incomplete type %s", texpr)
				expr.SetType(ir.TBuiltinInvalid)
				texpr = expr.Type()
			}
		}
	}
	return expr
}

func (c *checker) checkPointerTypeExpr(expr *ir.PointerTypeExpr) ir.Expr {
	expr.X = c.checkExpr2(expr.X, modeIndirectType)
	if tpunt := puntExprs(expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}
	ro := expr.Decl.Is(token.Val)
	tx := expr.X.Type()
	if tslice, ok := tx.(*ir.SliceType); ok {
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

func (c *checker) checkSliceTypeExpr(expr *ir.SliceTypeExpr) ir.Expr {
	expr.X = c.checkExpr2(expr.X, modeIndirectType)
	tx := expr.X.Type()
	if isUntyped(tx) {
		expr.T = tx
	} else {
		expr.T = ir.NewSliceType(tx, true, false)
	}
	return expr
}

func (c *checker) checkArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	if isUnresolvedExpr(expr.Size) {
		expr.Size = c.checkExpr2(expr.Size, modeCheck)
		expr.Size = c.finalizeExpr(expr.Size, nil)
	}
	expr.X = c.checkExpr(expr.X)
	if tpunt := puntExprs(expr.Size, expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}
	size := 0
	if !ir.IsIntegerType(expr.Size.Type()) {
		c.error(expr.Size.Pos(), "array size expects an integer type (got %s)", expr.Size.Type())
	} else if lit, ok := expr.Size.(*ir.BasicLit); !ok {
		c.error(expr.Size.Pos(), "array size is not a constant expression")
	} else if lit.NegatigeInteger() {
		c.error(expr.Size.Pos(), "array size cannot be negative")
	} else if lit.Zero() {
		c.error(expr.Size.Pos(), "array size cannot be zero")
	} else {
		size = int(lit.AsU64())
	}
	if size == 0 {
		expr.T = ir.TBuiltinInvalid
		return expr
	}
	tx := expr.X.Type()
	expr.T = ir.NewArrayType(tx, size)
	return expr
}

func (c *checker) checkFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	var params []ir.Field
	var tpunt ir.Type
	for i, param := range expr.Params {
		expr.Params[i].Type = c.checkRootTypeExpr(param.Type, true)
		tparam := expr.Params[i].Type.Type()
		params = append(params, ir.Field{Name: param.Name.Literal, T: tparam})
		tpunt = untyped(tparam, tpunt)
	}
	expr.Return.Type = c.checkRootTypeExpr(expr.Return.Type, false)
	tpunt = untyped(expr.Return.Type.Type(), tpunt)
	if tpunt != nil {
		expr.T = tpunt
		return expr
	}
	cabi := false
	if expr.ABI != nil {
		if expr.ABI.Literal == ir.CABI {
			cabi = true
		} else if !ir.IsValidABI(expr.ABI.Literal) {
			c.error(expr.ABI.Pos(), "unknown abi '%s'", expr.ABI.Literal)
			expr.T = ir.TBuiltinInvalid
			return expr
		}
	}
	expr.T = ir.NewFuncType(params, expr.Return.Type.Type(), cabi)
	return expr
}

func (c *checker) resolveIdent(expr *ir.Ident) (*ir.Symbol, bool) {
	ok := true
	sym := c.lookup(expr.Literal)
	if sym == nil {
		ok = false
		c.error(expr.Pos(), "undeclared identifier '%s'", expr.Literal)
	} else if c.mode != modeDot {
		if c.isTypeMode() {
			if sym.Kind != ir.TypeSymbol {
				ok = false
				c.error(expr.Pos(), "'%s' is not a type", sym.Name)
			}
		} else {
			if sym.Kind == ir.ModuleSymbol {
				ok = false
				c.error(expr.Pos(), "module '%s' cannot be used in an expression", sym.Name)
			} else if sym.Kind == ir.TypeSymbol {
				ok = false
				c.error(expr.Pos(), "type %s cannot be used in an expression", sym.T)
			}
		}
	}
	if ok {
		if !sym.Public && sym.ParentCUID() != c.object.parentCUID() {
			ok = false
			c.error(expr.Pos(), "'%s' is private", expr.Literal)
		}
	}
	return sym, ok
}

func (c *checker) checkIdent(expr *ir.Ident) ir.Expr {
	if c.step == 0 {
		if sym, ok := c.resolveIdent(expr); ok {
			expr.Sym = sym
		} else {
			expr.T = ir.TBuiltinInvalid
			return expr
		}
	} else if expr.T.Kind() == ir.TInvalid {
		return expr
	}

	sym := expr.Sym
	expr.T = sym.T

	if sym.IsBuiltin() && sym.IsConst() {
		return c.constMap[sym.Key]
	}

	c.tryAddDep(sym, expr.Pos())

	return expr
}
