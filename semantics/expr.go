package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func (v *typeChecker) visitType(expr ir.Expr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	expr = ir.VisitExpr(v, expr)
	v.exprMode = prevMode
	return expr
}

func (v *typeChecker) makeTypedExpr(expr ir.Expr, ttarget ir.Type) ir.Expr {
	x := ir.VisitExpr(v, expr)

	if ir.IsUntyped(x.Type()) || (ttarget != nil && ir.IsUntyped(ttarget)) {
		return x
	}

	if ir.IsCompilerType(x.Type()) {
		if ttarget != nil {
			v.tryMakeTypedLit(x, ttarget)
		} else {
			v.tryMakeDefaultTypedLit(x)
		}
	}

	tx := x.Type()

	switch x2 := x.(type) {
	case *ir.SliceExpr:
		if ir.IsIncompleteType(x.Type(), nil) {
			x2.T = ir.TBuiltinUntyped
		}
	case *ir.UnaryExpr:
		if ir.IsIncompleteType(x.Type(), nil) {
			x2.T = ir.TBuiltinUntyped
		}
	case *ir.AddressExpr:
		if ir.IsIncompleteType(x.Type(), nil) {
			x2.T = ir.TBuiltinUntyped
		}
	}

	if ir.IsUntyped(x.Type()) {
		v.c.errorExpr(expr, "expression has incomplete type %s", tx)
		return x
	}

	if ttarget != nil && x.Type().ImplicitCastOK(ttarget) {
		cast := &ir.CastExpr{}
		cast.X = x
		cast.T = ttarget
		return cast
	}

	return x
}

func (v *typeChecker) tryMakeTypedLit(expr ir.Expr, target ir.Type) bool {
	switch lit := expr.(type) {
	case *ir.BasicLit:
		if ir.IsTypeID(lit.T, ir.TBigInt, ir.TBigFloat) && ir.IsNumericType(target) {
			castResult := typeCastNumericLit(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				v.c.error(lit.Tok.Pos, "constant expression overflows %s", target)
			} else if castResult == numericCastTruncated {
				v.c.error(lit.Tok.Pos, "constant float expression is not compatible with %s", target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		} else if ir.IsUntypedPointer(expr.Type()) && ir.IsTypeID(target, ir.TSlice, ir.TPointer, ir.TFunc) {
			common.Assert(lit.Tok.ID == token.Null, "pointer literal should be null")
			switch ttarget := target.(type) {
			case *ir.SliceType:
				lit.T = ir.NewSliceType(ttarget.Elem, false, true)
			case *ir.PointerType:
				lit.T = ir.NewPointerType(ttarget.Underlying, false)
			case *ir.FuncType:
				lit.T = ir.NewFuncType(ttarget.Params, ttarget.Return, ttarget.C)
			default:
				panic(fmt.Sprintf("Unhandled target type %T", ttarget))
			}
		}
	case *ir.ArrayLit:
		if ir.IsTypeID(expr.Type(), ir.TArray) && ir.IsTypeID(target, ir.TArray) {
			tarray := expr.Type().(*ir.ArrayType)
			ttargetArray := target.(*ir.ArrayType)
			err := false
			for _, init := range lit.Initializers {
				if !v.tryMakeTypedLit(init, ttargetArray.Elem) {
					err = true
					break
				}
			}
			if !err && len(lit.Initializers) > 0 {
				tarray.Elem = lit.Initializers[0].Type()
			}
		}
	}

	return true
}

func (v *typeChecker) tryMakeDefaultTypedLit(expr ir.Expr) bool {
	texpr := expr.Type()
	if texpr.ID() == ir.TBigInt {
		return v.tryMakeTypedLit(expr, ir.TBuiltinInt32)
	} else if texpr.ID() == ir.TBigFloat {
		return v.tryMakeTypedLit(expr, ir.TBuiltinFloat64)
	} else if ir.IsUntypedPointer(texpr) {
		tpointer := ir.NewPointerType(ir.NewBasicType(ir.TUInt8), false)
		return v.tryMakeTypedLit(expr, tpointer)
	} else if texpr.ID() == ir.TArray {
		if lit, ok := expr.(*ir.ArrayLit); ok {
			err := false
			for _, init := range lit.Initializers {
				if !v.tryMakeDefaultTypedLit(init) {
					err = true
					break
				}
			}
			size := len(lit.Initializers)
			if !err && size > 0 {
				telem := lit.Initializers[0].Type()
				lit.T = ir.NewArrayType(telem, size)
			}
		}
	}
	return true
}

func tryDeref(expr ir.Expr) ir.Expr {
	var tres ir.Type
	switch t1 := expr.Type().(type) {
	case *ir.PointerType:
		switch t2 := t1.Underlying.(type) {
		case *ir.StructType:
			tres = t2
		case *ir.ArrayType:
			tres = t2
		case *ir.SliceType:
			tres = t2
		}
	}
	if tres != nil {
		star := token.Synthetic(token.Mul)
		starX := &ir.UnaryExpr{Op: star, X: expr}
		starX.T = tres
		return starX
	}
	return expr
}

func (v *typeChecker) VisitPointerTypeExpr(expr *ir.PointerTypeExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)

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

func (v *typeChecker) VisitArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	arraySize := 0

	if expr.Size != nil {
		prevMode := v.exprMode
		v.exprMode = exprModeNone
		size := v.makeTypedExpr(expr.Size, ir.TBuiltinInt32)
		v.exprMode = prevMode

		err := false

		if ir.IsUntyped(size.Type()) {
			err = true
		} else if size.Type().ID() != ir.TInt32 {
			v.c.error(expr.Size.Pos(), "array size must be of type %s (got %s)", ir.TInt32, size.Type())
			err = true
		} else if lit, ok := size.(*ir.BasicLit); !ok {
			v.c.error(expr.Size.Pos(), "array size is not a constant expression")
			err = true
		} else if lit.NegatigeInteger() {
			v.c.error(expr.Size.Pos(), "array size cannot be negative")
			err = true
		} else if lit.Zero() {
			v.c.error(expr.Size.Pos(), "array size cannot be zero")
			err = true
		} else {
			arraySize = int(lit.AsU64())
		}

		expr.Size = size

		if err {
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	x := ir.VisitExpr(v, expr.X)
	expr.X = x

	if x.Type().Equals(ir.TBuiltinUntyped) {
		expr.T = ir.TBuiltinUntyped
	} else if arraySize != 0 {
		expr.T = ir.NewArrayType(x.Type(), arraySize)
	} else {
		expr.T = ir.NewSliceType(x.Type(), true, false)
	}

	return expr
}

func (v *typeChecker) VisitFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	var tparams []ir.Type
	untyped := false
	for i, param := range expr.Params {
		expr.Params[i].Type = ir.VisitExpr(v, param.Type)
		tparam := expr.Params[i].Type.Type()
		tparams = append(tparams, tparam)
		if ir.IsUntyped(tparam) {
			untyped = true
		}
	}

	expr.Return.Type = ir.VisitExpr(v, expr.Return.Type)
	if ir.IsUntyped(expr.Return.Type.Type()) {
		untyped = true
	}

	if !untyped {
		c := v.checkCABI(expr.ABI)
		expr.T = ir.NewFuncType(tparams, expr.Return.Type.Type(), c)
	} else {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitIdent(expr *ir.Ident) ir.Expr {
	sym := expr.Sym
	if sym == nil {
		sym = v.c.lookup(expr.Literal)
	} else {
		expr.SetSymbol(nil)
	}

	err := false

	if sym == nil {
		v.c.error(expr.Pos(), "'%s' undefined", expr.Literal)
		err = true
	} else if sym.T == nil || sym.Untyped() {
		// Cycle or an error has already occurred
		err = true
	} else if v.exprMode != exprModeDot {
		if v.exprMode != exprModeType && sym.ID == ir.TypeSymbol {
			v.c.error(expr.Pos(), "type %s cannot be used in an expression", sym.T)
			err = true
		} else if v.exprMode == exprModeType && sym.ID != ir.TypeSymbol {
			v.c.error(expr.Pos(), "'%s' is not a type", sym.Name)
			err = true
		}
	}

	if err {
		expr.T = ir.TBuiltinUntyped
	} else {
		decl := v.c.topDecl()
		// Check symbol in other module is public (struct fields are exempted).
		if !sym.Public && sym.ModFQN() != decl.Symbol().ModFQN() && sym.Parent.ID != ir.FieldScope {
			v.c.error(expr.Pos(), "'%s' is not public", expr.Literal)
			expr.T = ir.TBuiltinUntyped
			err = true
		} else {
			expr.SetSymbol(sym)
		}
	}

	if !err && sym.ID == ir.ConstSymbol {
		return v.c.constExprs[sym]
	}

	return expr
}

func (v *typeChecker) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeDot
	x := ir.VisitExpr(v, expr.X)
	v.exprMode = prevMode
	x = tryDeref(x)

	if ir.IsUntyped(x.Type()) {
		// Do nothing
	} else {
		var scope *ir.Scope
		untyped := false

		switch tx := x.Type().(type) {
		case *ir.ModuleType:
			scope = tx.Scope
		case *ir.StructType:
			scope = tx.Scope
		case *ir.BasicType:
			if tx.ID() == ir.TUntyped {
				untyped = true
			}
		}

		if scope != nil {
			defer setScope(setScope(v.c, scope))
			v.VisitIdent(expr.Name)
			expr.T = expr.Name.Type()
		} else if !untyped {
			v.c.error(expr.X.Pos(), "type %s does not support field access", x.Type())
		}
	}

	expr.X = x

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	toType := ir.VisitExpr(v, expr.ToType)
	v.exprMode = prevMode

	err := false

	if ir.IsUntyped(toType.Type()) {
		err = true
	} else {
		sym := ir.ExprSymbol(toType)
		if sym != nil && sym.ID != ir.TypeSymbol {
			v.c.error(expr.ToType.Pos(), "'%s' is not a type", sym.Name)
			err = true
		}
	}

	x := ir.VisitExpr(v, expr.X)

	if ir.IsUntyped(x.Type()) {
		err = true
	}

	if !err {
		t1 := toType.Type()
		t2 := x.Type()

		if t2.ExplicitCastOK(t1) {
			expr.T = t1
		} else {
			v.c.error(expr.X.Pos(), "type %s cannot be cast to %s", t2, t1)
		}
	}

	expr.ToType = toType
	expr.X = x

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitLenExpr(expr *ir.LenExpr) ir.Expr {
	x := ir.VisitExpr(v, expr.X)
	x = tryDeref(x)

	switch tx := x.Type().(type) {
	case *ir.ArrayType:
		expr.T = ir.TBuiltinInt32
	case *ir.SliceType:
		expr.T = ir.TBuiltinInt32
	default:
		if !ir.IsUntyped(tx) {
			v.c.error(expr.X.Pos(), "type %s does not have a length", tx)
		}
	}

	expr.X = x

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeFunc
	x := ir.VisitExpr(v, expr.X)
	v.exprMode = prevMode

	if ir.IsUntyped(x.Type()) {
		// Do nothing
	} else if x.Type().ID() != ir.TFunc {
		v.c.errorExpr(expr.X, "expression is not callable (has type %s)", x.Type())
	} else {
		tfun := x.Type().(*ir.FuncType)

		if len(tfun.Params) != len(expr.Args) {
			v.c.error(expr.Lparen.Pos, "function takes %d argument(s) but called with %d", len(tfun.Params), len(expr.Args))
		} else {
			for i, arg := range expr.Args {
				expr.Args[i] = v.makeTypedExpr(arg, tfun.Params[i])
				tparam := tfun.Params[i]
				targ := expr.Args[i].Type()

				if !checkTypes(v.c, targ, tparam) {
					v.c.errorExpr(arg, "argument %d expects type %s (got type %s)", i, tparam, targ)
				}
			}
			expr.T = tfun.Return
		}
	}

	expr.X = x

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitAddressExpr(expr *ir.AddressExpr) ir.Expr {
	ro := expr.Decl.Is(token.Val)
	x := ir.VisitExpr(v, expr.X)

	if ir.IsUntyped(x.Type()) {
		// Do nothing
	} else if !x.Lvalue() {
		v.c.error(expr.X.Pos(), "expression is not an lvalue")
	} else if x.ReadOnly() && !ro {
		v.c.error(expr.X.Pos(), "expression is read-only")
	} else {
		if tslice, ok := x.Type().(*ir.SliceType); ok {
			if !tslice.Ptr {
				tslice.Ptr = true
				tslice.ReadOnly = ro
				expr.T = tslice
			} else {
				expr.T = ir.NewPointerType(tslice, ro)
			}
		} else {
			expr.T = ir.NewPointerType(x.Type(), ro)
		}
	}

	expr.X = x

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitIndexExpr(expr *ir.IndexExpr) ir.Expr {
	x := v.makeTypedExpr(expr.X, nil)
	index := v.makeTypedExpr(expr.Index, nil)

	var telem ir.Type
	untyped := false

	expr.X = tryDeref(x)
	switch tx := x.Type().(type) {
	case *ir.ArrayType:
		telem = tx.Elem
	case *ir.SliceType:
		telem = tx.Elem
	case *ir.BasicType:
		if tx.ID() == ir.TUntyped {
			untyped = true
		}
	}

	if telem != nil {
		if !ir.IsUntyped(index.Type()) {
			if !ir.IsIntegerType(index.Type()) {
				v.c.error(expr.Index.Pos(), "type %s cannot be used as an index", index.Type())
			} else {
				expr.T = telem
			}
		}
	} else if !untyped {
		v.c.error(expr.X.Pos(), "type %s cannot be indexed", x.Type())
	}

	expr.X = x
	expr.Index = index

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitSliceExpr(expr *ir.SliceExpr) ir.Expr {
	x := v.makeTypedExpr(expr.X, nil)
	x = tryDeref(x)

	var start ir.Expr
	var end ir.Expr

	if expr.Start != nil {
		start = v.makeTypedExpr(expr.Start, nil)
	}

	if expr.End != nil {
		end = v.makeTypedExpr(expr.End, nil)
	}

	tbase := x.Type()
	ro := x.ReadOnly()
	var telem ir.Type

	switch t := tbase.(type) {
	case *ir.ArrayType:
		telem = t.Elem
	case *ir.SliceType:
		telem = t.Elem
		ro = t.ReadOnly
	case *ir.PointerType:
		telem = t.Underlying
		ro = t.ReadOnly
	}

	if telem != nil {
		err := false

		if start != nil {
			if !ir.IsUntyped(start.Type()) && !ir.IsIntegerType(start.Type()) {
				v.c.error(expr.Start.Pos(), "type %s cannot be used as slice index", start.Type())
				err = true
			}
		}

		if end != nil {
			if !ir.IsUntyped(end.Type()) && !ir.IsIntegerType(end.Type()) {
				v.c.error(expr.End.Pos(), "type %s cannot be used as slice index", end.Type())
				err = true
			}
		}

		if !err {
			if start == nil {
				tstart := ir.TBuiltinInt32
				if end != nil {
					tstart = end.Type()
				}
				start = createDefaultBasicLit(tstart)
			} else if start.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: start}
				cast.T = ir.TBuiltinInt32
				start = cast
			}

			if end == nil {
				if tbase.ID() == ir.TPointer {
					v.c.error(expr.Colon.Pos, "end index is required when slicing type %s", tbase)
					err = true
				} else {
					len := &ir.LenExpr{
						Len: token.Synthetic(token.Lenof),
						X:   x,
					}
					len.T = ir.TBuiltinInt32
					end = len
				}
			} else if end.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: end}
				cast.T = ir.TBuiltinInt32
				end = cast
			}
		}

		if !err {
			expr.T = ir.NewSliceType(telem, ro, false)
		}
	} else if tbase.ID() != ir.TUntyped {
		v.c.errorExpr(expr.X, "type %s cannot be sliced", tbase)
	}

	expr.X = x
	expr.Start = start
	expr.End = end

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}
