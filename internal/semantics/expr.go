package semantics

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) makeTypedExpr(expr ir.Expr, ttarget ir.Type) ir.Expr {
	expr = c.typeswitchExpr(expr)

	if ir.IsUntyped(expr.Type()) || (ttarget != nil && ir.IsUntyped(ttarget)) {
		return expr
	}

	if ir.IsCompilerType(expr.Type()) {
		if ttarget != nil {
			c.tryMakeTypedLit(expr, ttarget)
		} else {
			c.tryMakeDefaultTypedLit(expr)
		}
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

	if checkIncomplete && ir.IsIncompleteType(texpr, nil) {
		c.errorNode(expr, "expression has incomplete type %s", texpr)
		expr.SetType(ir.TBuiltinUntyped)
		return expr
	}

	if ttarget != nil {
		if ir.IsImplicitCastNeeded(texpr, ttarget) {
			cast := &ir.CastExpr{}
			cast.SetRange(expr.Pos(), expr.EndPos())
			cast.X = expr
			cast.T = ttarget
			return cast
		}
	}

	return expr
}

func (c *checker) tryMakeTypedLit(expr ir.Expr, target ir.Type) bool {
	switch lit := expr.(type) {
	case *ir.BasicLit:
		if ir.IsTypeID(lit.T, ir.TBigInt, ir.TBigFloat) && ir.IsNumericType(target) {
			castResult := typeCastNumericLit(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Pos(), "constant expression overflows %s", target)
			} else if castResult == numericCastTruncated {
				c.error(lit.Pos(), "constant float expression is not compatible with %s", target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		} else if ir.IsUntypedPointer(expr.Type()) && ir.IsTypeID(target, ir.TSlice, ir.TPointer, ir.TFunc) {
			common.Assert(lit.Tok == token.Null, "pointer literal should be null")
			expr.SetType(target)
		}
	case *ir.ArrayLit:
		if ir.IsTypeID(expr.Type(), ir.TArray) && ir.IsTypeID(target, ir.TArray) {
			tarray := ir.ToBaseType(expr.Type()).(*ir.ArrayType)
			ttargetArray := ir.ToBaseType(target).(*ir.ArrayType)
			err := false
			for _, init := range lit.Initializers {
				if !c.tryMakeTypedLit(init, ttargetArray.Elem) {
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

func (c *checker) tryMakeDefaultTypedLit(expr ir.Expr) bool {
	texpr := expr.Type()
	if texpr.ID() == ir.TBigInt {
		return c.tryMakeTypedLit(expr, ir.TBuiltinInt32)
	} else if texpr.ID() == ir.TBigFloat {
		return c.tryMakeTypedLit(expr, ir.TBuiltinFloat64)
	} else if ir.IsUntypedPointer(texpr) {
		tpointer := ir.NewPointerType(ir.NewBasicType(ir.TUInt8), false)
		return c.tryMakeTypedLit(expr, tpointer)
	} else if texpr.ID() == ir.TArray {
		if lit, ok := expr.(*ir.ArrayLit); ok {
			err := false
			for _, init := range lit.Initializers {
				if !c.tryMakeDefaultTypedLit(init) {
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
	switch t1 := ir.ToBaseType(expr.Type()).(type) {
	case *ir.PointerType:
		switch t2 := ir.ToBaseType(t1.Elem).(type) {
		case *ir.StructType:
			tres = t2
		case *ir.ArrayType:
			tres = t2
		case *ir.SliceType:
			tres = t2
		}
	}
	if tres != nil {
		deref := &ir.UnaryExpr{Op: token.Deref, X: expr}
		deref.SetRange(expr.Pos(), expr.EndPos())
		deref.T = tres
		return deref
	}

	return expr
}

func (c *checker) checkStructLit(expr *ir.StructLit) ir.Expr {
	expr.Name = c.checkTypeExpr(expr.Name, true, true)
	tname := expr.Name.Type()

	if ir.IsUntyped(tname) {
		expr.T = ir.TBuiltinUntyped
		return expr
	} else if nameSym := ir.ExprSymbol(expr.Name); nameSym != nil {
		if !nameSym.IsType() || !ir.IsTypeID(nameSym.T, ir.TStruct) {
			c.error(expr.Name.Pos(), "'%s' is not a struct", nameSym.Name)
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	tstruct := ir.ToBaseType(tname).(*ir.StructType)
	expr.Args = c.checkArgumentList(tstruct, expr.Args, tstruct.Fields, true)
	expr.T = tname
	return expr
}

func (c *checker) checkArrayLit(expr *ir.ArrayLit) ir.Expr {
	texpr := ir.TBuiltinUntyped
	tbackup := ir.TBuiltinUntyped

	for i, init := range expr.Initializers {
		init = c.typeswitchExpr(init)
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
			if !c.tryMakeTypedLit(init, texpr) {
				break
			}
		}
	}

	if !ir.IsUntyped(texpr) {
		for _, init := range expr.Initializers {
			if ir.IsUntyped(init.Type()) {
				texpr = ir.TBuiltinUntyped
			} else if !checkTypes(texpr, init.Type()) {
				c.error(init.Pos(), "array elements must be of the same type (expected type %s, got type %s)", texpr, init.Type())
				texpr = ir.TBuiltinUntyped
				break
			}
		}
	}

	if len(expr.Initializers) == 0 {
		c.error(expr.Pos(), "array literal cannot have 0 elements")
		texpr = ir.TBuiltinUntyped
	}

	if ir.IsUntyped(texpr) {
		expr.T = texpr
	} else {
		expr.T = ir.NewArrayType(texpr, len(expr.Initializers))
	}

	return expr
}

func (c *checker) checkDotExpr(expr *ir.DotExpr) ir.Expr {
	prevMode := c.exprMode
	c.exprMode = exprModeDot
	expr.X = c.typeswitchExpr(expr.X)
	c.exprMode = prevMode

	expr.X = tryDeref(expr.X)
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		// Do nothing
	} else if ir.IsIncompleteType(tx, nil) {
		c.errorNode(expr.X, "expression has incomplete type %s", tx)
	} else if !ir.IsUntyped(tx) {
		var scope *ir.Scope

		switch tx2 := ir.ToBaseType(tx).(type) {
		case *ir.ModuleType:
			scope = tx2.Scope
		case *ir.StructType:
			scope = tx2.Scope
		default:
			c.error(expr.X.Pos(), "type %s does not support field access", tx)
		}

		if scope != nil {
			defer setScope(setScope(c, scope))
			c.checkIdent(expr.Name)
			expr.T = expr.Name.Type()
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkIndexExpr(expr *ir.IndexExpr) ir.Expr {
	expr.X = c.makeTypedExpr(expr.X, nil)
	expr.Index = c.makeTypedExpr(expr.Index, nil)
	expr.X = tryDeref(expr.X)

	var telem ir.Type
	untyped := false

	if ir.IsUntyped(expr.X.Type()) {
		untyped = true
	} else {
		switch tx := ir.ToBaseType(expr.X.Type()).(type) {
		case *ir.ArrayType:
			telem = tx.Elem
		case *ir.SliceType:
			telem = tx.Elem
		}
	}

	if telem != nil {
		tindex := expr.Index.Type()
		if !ir.IsUntyped(tindex) {
			if !ir.IsIntegerType(tindex) {
				c.error(expr.Index.Pos(), "type %s cannot be used as an index", tindex)
			} else {
				expr.T = telem
			}
		}
	} else if !untyped {
		c.error(expr.X.Pos(), "type %s cannot be indexed", expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkSliceExpr(expr *ir.SliceExpr) ir.Expr {
	expr.X = c.makeTypedExpr(expr.X, nil)
	expr.X = tryDeref(expr.X)

	if expr.Start != nil {
		expr.Start = c.makeTypedExpr(expr.Start, nil)
	}

	if expr.End != nil {
		expr.End = c.makeTypedExpr(expr.End, nil)
	}

	tx := expr.X.Type()
	ro := expr.X.ReadOnly()
	var telem ir.Type

	switch t := ir.ToBaseType(tx).(type) {
	case *ir.ArrayType:
		telem = t.Elem
	case *ir.SliceType:
		telem = t.Elem
		ro = t.ReadOnly
	case *ir.PointerType:
		telem = t.Elem
		ro = t.ReadOnly
	}

	if telem != nil {
		err := false

		if expr.Start != nil {
			tstart := expr.Start.Type()
			if !ir.IsUntyped(tstart) && !ir.IsIntegerType(tstart) {
				c.error(expr.Start.Pos(), "type %s cannot be used as slice index", tstart)
				err = true
			}
		}

		if expr.End != nil {
			tend := expr.End.Type()
			if !ir.IsUntyped(tend) && !ir.IsIntegerType(tend) {
				c.error(expr.End.Pos(), "type %s cannot be used as slice index", tend)
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
				if tx.ID() == ir.TPointer {
					c.errorNode(expr, "end index is required when slicing type %s", tx)
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
	} else if !ir.IsUntyped(tx) {
		c.errorNode(expr.X, "type %s cannot be sliced", tx)
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkFuncCall(expr *ir.FuncCall) ir.Expr {
	prevMode := c.exprMode
	c.exprMode = exprModeFunc
	expr.X = c.typeswitchExpr(expr.X)
	c.exprMode = prevMode

	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		// Do nothing
	} else if tx.ID() != ir.TFunc {
		c.errorNode(expr.X, "expression is not callable (has type %s)", tx)
	} else {
		tfun := ir.ToBaseType(tx).(*ir.FuncType)
		expr.Args = c.checkArgumentList(tx, expr.Args, tfun.Params, false)
		expr.T = tfun.Return
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkArgumentList(tobj ir.Type, args []*ir.ArgExpr, fields []ir.Field, autofill bool) []*ir.ArgExpr {
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
					if arg.Name.Literal == field.Name {
						fieldIndex = i
						break
					}
				}
				if fieldIndex < 0 {
					c.errorNode(arg, "unknown named argument '%s'", arg.Name.Literal)
				}
			} else if named {
				c.errorNode(arg, "positioned argument after named argument is not allowed")
				mixed = true
			} else if argIndex < len(fields) {
				fieldIndex = argIndex
			} else {
				break
			}

			if fieldIndex >= 0 {
				field := fields[fieldIndex]
				arg.Value = c.makeTypedExpr(arg.Value, field.T)
				if existing := argsRes[fieldIndex]; existing != nil {
					// This is only possible if the current argument is named
					c.errorNode(arg, "duplicate arguments for '%s' at position %d", arg.Name.Literal, fieldIndex+1)
				} else if !checkTypes(arg.Value.Type(), field.T) {
					kind := "parameter"
					if tobj.ID() == ir.TStruct {
						kind = "field"
					}
					if arg.Name != nil {
						c.error(arg.Pos(), "%s '%s' at position %d has type %s (got %s)", kind, arg.Name.Literal, fieldIndex+1, field.T, arg.Value.Type())
					} else {
						c.error(arg.Pos(), "%s at position %d has type %s (got %s)", kind, fieldIndex+1, field.T, arg.Value.Type())
					}
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
					c.error(endPos, "no argument for '%s' at position %d", field.Name, fieldIndex+1)
				}
			}
		}
	} else if len(args) > len(fields) {
		c.error(endPos, "too many arguments (got %d but expected %d)", len(args), len(fields))
	} else if len(args) < len(fields) {
		c.error(endPos, "too few arguments (got %d but expected %d)", len(args), len(fields))
	}

	return argsRes
}

func (c *checker) checkCastExpr(expr *ir.CastExpr) ir.Expr {
	prevMode := c.exprMode
	c.exprMode = exprModeType
	expr.ToType = c.typeswitchExpr(expr.ToType)
	c.exprMode = prevMode

	err := false

	if ir.IsUntyped(expr.ToType.Type()) {
		err = true
	} else {
		sym := ir.ExprSymbol(expr.ToType)
		if sym != nil && !sym.IsType() {
			c.error(expr.ToType.Pos(), "'%s' is not a type", sym.Name)
			err = true
		}
	}

	expr.X = c.typeswitchExpr(expr.X)

	if ir.IsUntyped(expr.X.Type()) {
		err = true
	}

	if !err {
		t1 := expr.ToType.Type()
		t2 := expr.X.Type()

		if t2.CastableTo(t1) {
			expr.T = t1
		} else {
			c.error(expr.X.Pos(), "type %s cannot be cast to %s", t2, t1)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkLenExpr(expr *ir.LenExpr) ir.Expr {
	expr.X = c.typeswitchExpr(expr.X)
	expr.X = tryDeref(expr.X)

	if !ir.IsUntyped(expr.X.Type()) {
		switch tx := ir.ToBaseType(expr.X.Type()).(type) {
		case *ir.ArrayType:
			expr.T = ir.TBuiltinInt32
		case *ir.SliceType:
			expr.T = ir.TBuiltinInt32
		default:
			c.error(expr.X.Pos(), "type %s does not have a length", tx)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (c *checker) checkSizeExpr(expr *ir.SizeExpr) ir.Expr {
	expr.X = c.checkTypeExpr(expr.X, true, true)
	expr.T = ir.TBuiltinUntyped
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		return expr
	}

	size := c.target.Sizeof(tx)
	return createIntLit(size, ir.NewBasicType(ir.TBigInt))
}

func createIntLit(val int, t ir.Type) *ir.BasicLit {
	lit := &ir.BasicLit{Tok: token.Integer, Value: strconv.FormatInt(int64(val), 10)}
	lit.T = t
	if val == 0 {
		lit.Raw = ir.BigIntZero
	} else {
		lit.Raw = big.NewInt(int64(val))
	}
	return lit
}

func createDefaultLit(t ir.Type) ir.Expr {
	if t.ID() == ir.TStruct {
		tstruct := ir.ToBaseType(t).(*ir.StructType)
		lit := &ir.StructLit{}
		for _, field := range tstruct.Fields {
			arg := &ir.ArgExpr{}
			arg.Value = createDefaultLit(field.T)
			lit.Args = append(lit.Args, arg)
		}
		lit.T = t
		return lit
	} else if t.ID() == ir.TArray {
		tarray := ir.ToBaseType(t).(*ir.ArrayType)
		lit := &ir.ArrayLit{}
		for i := 0; i < tarray.Size; i++ {
			init := createDefaultLit(tarray.Elem)
			lit.Initializers = append(lit.Initializers, init)
		}
		lit.T = t
		return lit
	}
	return createDefaultBasicLit(t)
}

func createDefaultBasicLit(t ir.Type) *ir.BasicLit {
	var lit *ir.BasicLit
	if ir.IsTypeID(t, ir.TBool) {
		lit = &ir.BasicLit{Tok: token.False, Value: token.False.String()}
		lit.T = t
	} else if ir.IsTypeID(t, ir.TUInt64, ir.TInt64, ir.TUInt32, ir.TInt32, ir.TUInt16, ir.TInt16, ir.TUInt8, ir.TInt8) {
		lit = createIntLit(0, t)
	} else if ir.IsTypeID(t, ir.TFloat64, ir.TFloat32) {
		lit = &ir.BasicLit{Tok: token.Float, Value: "0"}
		lit.Raw = ir.BigFloatZero
		lit.T = t
	} else if ir.IsTypeID(t, ir.TSlice) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		lit.T = t
	} else if ir.IsTypeID(t, ir.TPointer) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		lit.T = t
	} else if ir.IsTypeID(t, ir.TFunc) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		lit.T = t
	} else {
		panic(fmt.Sprintf("Unhandled init value for type %s", t))
	}
	return lit
}
