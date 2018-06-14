package semantics

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (v *typeChecker) visitType(expr ir.Expr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	expr = ir.VisitExpr(v, expr)
	v.exprMode = prevMode
	return expr
}

func (v *typeChecker) makeTypedExpr(expr ir.Expr, ttarget ir.Type) ir.Expr {
	expr = ir.VisitExpr(v, expr)

	if ir.IsUntyped(expr.Type()) || (ttarget != nil && ir.IsUntyped(ttarget)) {
		return expr
	}

	if ir.IsCompilerType(expr.Type()) {
		if ttarget != nil {
			v.tryMakeTypedLit(expr, ttarget)
		} else {
			v.tryMakeDefaultTypedLit(expr)
		}
	}

	texpr := expr.Type()

	switch expr2 := expr.(type) {
	case *ir.SliceExpr:
		if ir.IsIncompleteType(texpr, nil) {
			expr2.T = ir.TBuiltinUntyped
		}
	case *ir.UnaryExpr:
		if ir.IsIncompleteType(texpr, nil) {
			expr2.T = ir.TBuiltinUntyped
		}
	case *ir.AddressExpr:
		if ir.IsIncompleteType(texpr, nil) {
			expr2.T = ir.TBuiltinUntyped
		}
	}

	if ir.IsUntyped(expr.Type()) {
		v.c.errorNode(expr, "expression has incomplete type %s", texpr)
		return expr
	}

	if ttarget != nil && texpr.ImplicitCastOK(ttarget) {
		cast := &ir.CastExpr{}
		cast.SetRange(expr.Pos(), expr.EndPos())
		cast.X = expr
		cast.T = ttarget
		return cast
	}

	return expr
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
				v.c.error(lit.Pos(), "constant expression overflows %s", target)
			} else if castResult == numericCastTruncated {
				v.c.error(lit.Pos(), "constant float expression is not compatible with %s", target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		} else if ir.IsUntypedPointer(expr.Type()) && ir.IsTypeID(target, ir.TSlice, ir.TPointer, ir.TFunc) {
			common.Assert(lit.Tok == token.Null, "pointer literal should be null")
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
		starX := &ir.UnaryExpr{Op: token.Mul, X: expr}
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
		expr.Size = v.makeTypedExpr(expr.Size, ir.TBuiltinInt32)
		v.exprMode = prevMode

		err := false

		size := expr.Size
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

		if err {
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	expr.X = ir.VisitExpr(v, expr.X)
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

func (v *typeChecker) VisitFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	var params []ir.Field
	untyped := false
	for i, param := range expr.Params {
		expr.Params[i].Type = ir.VisitExpr(v, param.Type)
		tparam := expr.Params[i].Type.Type()
		params = append(params, ir.Field{Sym: param.Sym, T: tparam})
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
		expr.T = ir.NewFuncType(params, expr.Return.Type.Type(), c)
	} else {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	expr.ToType = ir.VisitExpr(v, expr.ToType)
	v.exprMode = prevMode

	err := false

	if ir.IsUntyped(expr.ToType.Type()) {
		err = true
	} else {
		sym := ir.ExprSymbol(expr.ToType)
		if sym != nil && sym.ID != ir.TypeSymbol {
			v.c.error(expr.ToType.Pos(), "'%s' is not a type", sym.Name)
			err = true
		}
	}

	expr.X = ir.VisitExpr(v, expr.X)

	if ir.IsUntyped(expr.X.Type()) {
		err = true
	}

	if !err {
		t1 := expr.ToType.Type()
		t2 := expr.X.Type()

		if t2.ExplicitCastOK(t1) {
			expr.T = t1
		} else {
			v.c.error(expr.X.Pos(), "type %s cannot be cast to %s", t2, t1)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitLenExpr(expr *ir.LenExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)
	expr.X = tryDeref(expr.X)

	switch tx := expr.X.Type().(type) {
	case *ir.ArrayType:
		expr.T = ir.TBuiltinInt32
	case *ir.SliceType:
		expr.T = ir.TBuiltinInt32
	default:
		if !ir.IsUntyped(tx) {
			v.c.error(expr.X.Pos(), "type %s does not have a length", tx)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitSizeExpr(expr *ir.SizeExpr) ir.Expr {
	expr.X = v.visitType(expr.X)
	expr.T = ir.TBuiltinUntyped
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		return expr
	}

	if tx.ID() == ir.TModule || tx.ID() == ir.TVoid {
		v.c.errorNode(expr.X, "type %s does not have a size", tx)
		return expr
	}

	size := v.c.target.Sizeof(tx)
	return createIntLit(size, ir.TBigInt)
}

func (v *typeChecker) VisitAddressExpr(expr *ir.AddressExpr) ir.Expr {
	ro := expr.Decl.Is(token.Val)
	expr.X = ir.VisitExpr(v, expr.X)
	x := expr.X

	if ir.IsUntyped(x.Type()) {
		// Do nothing
	} else if !x.Lvalue() {
		v.c.error(x.Pos(), "expression is not an lvalue")
	} else if x.ReadOnly() && !ro {
		v.c.error(x.Pos(), "expression is read-only")
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

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitIndexExpr(expr *ir.IndexExpr) ir.Expr {
	expr.X = v.makeTypedExpr(expr.X, nil)
	expr.Index = v.makeTypedExpr(expr.Index, nil)
	expr.X = tryDeref(expr.X)

	var telem ir.Type
	untyped := false

	switch tx := expr.X.Type().(type) {
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
		tindex := expr.Index.Type()
		if !ir.IsUntyped(tindex) {
			if !ir.IsIntegerType(tindex) {
				v.c.error(expr.Index.Pos(), "type %s cannot be used as an index", tindex)
			} else {
				expr.T = telem
			}
		}
	} else if !untyped {
		v.c.error(expr.X.Pos(), "type %s cannot be indexed", expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitSliceExpr(expr *ir.SliceExpr) ir.Expr {
	expr.X = v.makeTypedExpr(expr.X, nil)
	expr.X = tryDeref(expr.X)

	if expr.Start != nil {
		expr.Start = v.makeTypedExpr(expr.Start, nil)
	}

	if expr.End != nil {
		expr.End = v.makeTypedExpr(expr.End, nil)
	}

	tbase := expr.X.Type()
	ro := expr.X.ReadOnly()
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

		if expr.Start != nil {
			tstart := expr.Start.Type()
			if !ir.IsUntyped(tstart) && !ir.IsIntegerType(tstart) {
				v.c.error(expr.Start.Pos(), "type %s cannot be used as slice index", tstart)
				err = true
			}
		}

		if expr.End != nil {
			tend := expr.End.Type()
			if !ir.IsUntyped(tend) && !ir.IsIntegerType(tend) {
				v.c.error(expr.End.Pos(), "type %s cannot be used as slice index", tend)
				err = true
			}
		}

		if !err {
			if expr.Start == nil {
				tstart := ir.TBuiltinInt32
				if expr.End != nil {
					tstart = expr.End.Type()
				}
				expr.Start = createDefaultBasicLit(tstart)
			} else if expr.Start.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: expr.Start}
				cast.SetRange(expr.Start.Pos(), expr.Start.EndPos())
				cast.T = ir.TBuiltinInt32
				expr.Start = cast
			}

			if expr.End == nil {
				if tbase.ID() == ir.TPointer {
					v.c.errorNode(expr, "end index is required when slicing type %s", tbase)
					err = true
				} else {
					len := &ir.LenExpr{
						X: expr.X,
					}
					len.T = ir.TBuiltinInt32
					expr.End = len
				}
			} else if expr.End.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: expr.End}
				cast.SetRange(expr.End.Pos(), expr.End.EndPos())
				cast.T = ir.TBuiltinInt32
				expr.End = cast
			}
		}

		if !err {
			expr.T = ir.NewSliceType(telem, ro, false)
		}
	} else if tbase.ID() != ir.TUntyped {
		v.c.errorNode(expr.X, "type %s cannot be sliced", tbase)
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) visitArgumentList(args []*ir.ArgExpr, fields []ir.Field, autofill bool) []*ir.ArgExpr {
	named := false
	mixed := false
	endPos := token.NoPosition
	argsRes := make([]*ir.ArgExpr, len(fields))

	if len(args) > 0 {
		endPos = args[len(args)-1].EndPos()

		for argIndex, arg := range args {
			fieldIndex := -1

			if arg.Name != nil {
				named = true
				for i, field := range fields {
					if arg.Name.Literal == field.Sym.Name {
						fieldIndex = i
						break
					}
				}
				if fieldIndex < 0 {
					v.c.errorNode(arg, "unknown named argument '%s'", arg.Name.Literal)
				}
			} else if named {
				v.c.errorNode(arg, "positioned argument after named argument is not allowed")
				mixed = true
			} else if argIndex < len(fields) {
				fieldIndex = argIndex
			} else {
				break
			}

			if fieldIndex >= 0 {
				field := fields[fieldIndex]
				arg.Value = v.makeTypedExpr(arg.Value, field.T)
				if existing := argsRes[fieldIndex]; existing != nil {
					// This is only possible if the current argument is named
					v.c.errorNode(arg, "duplicate arguments for '%s' at position %d", arg.Name.Literal, fieldIndex+1)
				} else if !checkTypes(v.c, arg.Value.Type(), field.T) {
					v.c.errorNode(arg, "type mismatch (got %s but expected %s)", arg.Value.Type(), field.T)
				}
				argsRes[fieldIndex] = arg
			}
		}
	} else if autofill {
		named = true
	}

	if named {
		if autofill {
			for fieldIndex, field := range fields {
				if argsRes[fieldIndex] == nil {
					arg := &ir.ArgExpr{}
					arg.Value = createDefaultLit(field.T)
					argsRes[fieldIndex] = arg
				}
			}
		} else if !mixed {
			for fieldIndex, field := range fields {
				if argsRes[fieldIndex] == nil {
					v.c.error(endPos, "no argument for '%s' at position %d", field.Sym.Name, fieldIndex+1)
				}
			}
		}
	} else if len(args) > len(fields) {
		v.c.error(endPos, "too many arguments (got %d but expected %d)", len(args), len(fields))
	} else if len(args) < len(fields) {
		v.c.error(endPos, "too few arguments (got %d but expected %d)", len(args), len(fields))
	}

	return argsRes
}

func (v *typeChecker) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeFunc
	expr.X = ir.VisitExpr(v, expr.X)
	v.exprMode = prevMode

	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		// Do nothing
	} else if tx.ID() != ir.TFunc {
		v.c.errorNode(expr.X, "expression is not callable (has type %s)", tx)
	} else {
		tfun := tx.(*ir.FuncType)
		expr.Args = v.visitArgumentList(expr.Args, tfun.Params, false)
		expr.T = tfun.Return
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitStructLit(expr *ir.StructLit) ir.Expr {
	expr.Name = v.visitType(expr.Name)
	tname := expr.Name.Type()

	if ir.IsUntyped(tname) {
		expr.T = ir.TBuiltinUntyped
		return expr
	} else if nameSym := ir.ExprSymbol(expr.Name); nameSym != nil {
		if nameSym.ID != ir.TypeSymbol || nameSym.T.ID() != ir.TStruct {
			v.c.error(expr.Name.Pos(), "'%s' is not a struct", nameSym.Name)
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	tstruct := tname.(*ir.StructType)
	expr.Args = v.visitArgumentList(expr.Args, tstruct.Fields, true)
	expr.T = tstruct
	return expr
}

func (v *typeChecker) VisitArrayLit(expr *ir.ArrayLit) ir.Expr {
	texpr := ir.TBuiltinUntyped
	tbackup := ir.TBuiltinUntyped

	for i, init := range expr.Initializers {
		init = ir.VisitExpr(v, init)
		expr.Initializers[i] = init

		if ir.IsUntyped(texpr) && !ir.IsCompilerType(init.Type()) {
			texpr = init.Type()
		}
		if ir.IsUntyped(tbackup) && !ir.IsUntyped(init.Type()) {
			tbackup = init.Type()
		}
	}

	if ir.IsUntyped(texpr) {
		texpr = tbackup
	} else {
		for _, init := range expr.Initializers {
			if !v.tryMakeTypedLit(init, texpr) {
				break
			}
		}
	}

	if !ir.IsUntyped(texpr) {
		for _, init := range expr.Initializers {
			if ir.IsUntyped(init.Type()) {
				texpr = ir.TBuiltinUntyped
			} else if !checkTypes(v.c, texpr, init.Type()) {
				v.c.error(init.Pos(), "array elements must be of the same type (expected type %s, got type %s)", texpr, init.Type())
				texpr = ir.TBuiltinUntyped
				break
			}
		}
	}

	if len(expr.Initializers) == 0 {
		v.c.error(expr.Pos(), "array literal cannot have 0 elements")
		texpr = ir.TBuiltinUntyped
	}

	if ir.IsUntyped(texpr) {
		expr.T = texpr
	} else {
		expr.T = ir.NewArrayType(texpr, len(expr.Initializers))
	}

	return expr
}

func createIntLit(val int, tid ir.TypeID) *ir.BasicLit {
	lit := &ir.BasicLit{Tok: token.Integer, Value: strconv.FormatInt(int64(val), 10)}
	lit.T = ir.NewBasicType(tid)
	if val == 0 {
		lit.Raw = ir.BigIntZero
	} else {
		lit.Raw = big.NewInt(int64(val))
	}
	return lit
}

func createDefaultLit(t ir.Type) ir.Expr {
	if t.ID() == ir.TStruct {
		tstruct := t.(*ir.StructType)
		lit := &ir.StructLit{}
		lit.T = t
		for _, field := range tstruct.Fields {
			arg := &ir.ArgExpr{}
			arg.Value = createDefaultLit(field.T)
			lit.Args = append(lit.Args, arg)
		}
		return lit
	} else if t.ID() == ir.TArray {
		tarray := t.(*ir.ArrayType)
		lit := &ir.ArrayLit{}
		lit.T = tarray
		for i := 0; i < tarray.Size; i++ {
			init := createDefaultLit(tarray.Elem)
			lit.Initializers = append(lit.Initializers, init)
		}
		return lit
	}
	return createDefaultBasicLit(t)
}

func createDefaultBasicLit(t ir.Type) *ir.BasicLit {
	var lit *ir.BasicLit
	if ir.IsTypeID(t, ir.TBool) {
		lit = &ir.BasicLit{Tok: token.False, Value: token.False.String()}
		lit.T = ir.NewBasicType(ir.TBool)
	} else if ir.IsTypeID(t, ir.TUInt64, ir.TInt64, ir.TUInt32, ir.TInt32, ir.TUInt16, ir.TInt16, ir.TUInt8, ir.TInt8) {
		lit = createIntLit(0, t.ID())
	} else if ir.IsTypeID(t, ir.TFloat64, ir.TFloat32) {
		lit = &ir.BasicLit{Tok: token.Float, Value: "0"}
		lit.Raw = ir.BigFloatZero
		lit.T = ir.NewBasicType(t.ID())
	} else if ir.IsTypeID(t, ir.TSlice) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		slice := t.(*ir.SliceType)
		lit.T = ir.NewSliceType(slice.Elem, true, true)
	} else if ir.IsTypeID(t, ir.TPointer) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		ptr := t.(*ir.PointerType)
		lit.T = ir.NewPointerType(ptr.Underlying, true)
	} else if ir.IsTypeID(t, ir.TFunc) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		fun := t.(*ir.FuncType)
		lit.T = ir.NewFuncType(fun.Params, fun.Return, fun.C)
	} else if !ir.IsTypeID(t, ir.TUntyped) {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID()))
	}
	return lit
}
