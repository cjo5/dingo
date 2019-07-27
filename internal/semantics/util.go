package semantics

import (
	"github.com/cjo5/dingo/internal/ir"
)

func isTypeMismatch(t1 ir.Type, t2 ir.Type) bool {
	if isUntyped(t1) || isUntyped(t2) {
		return false
	}
	return !t1.Equals(t2)
}

func puntExprs(exprs ...ir.Expr) ir.Type {
	var t ir.Type
	for _, expr := range exprs {
		if expr != nil {
			t = untyped(expr.Type(), t)
		}
	}
	return t
}

func untyped(t ir.Type, prev ir.Type) ir.Type {
	if prev == nil || prev.Kind() == ir.TUnknown {
		if isUntyped(t) {
			return t
		}
	}
	return prev
}

func untypedExpr(expr ir.Expr, prev ir.Type) ir.Type {
	texpr := expr.Type()
	return untyped(texpr, prev)
}

func isUntyped(t ir.Type) bool {
	return isTypeOneOf(t, ir.TUnknown, ir.TInvalid)
}

func isUntypedExpr(expr ir.Expr) bool {
	texpr := expr.Type()
	return isUntyped(texpr)
}

func isUnknownType(t ir.Type) bool {
	return t.Kind() == ir.TUnknown
}

func isUnknownExprType(expr ir.Expr) bool {
	texpr := expr.Type()
	return isUnknownType(texpr)
}

func isUnknownExprsType(exprs ...ir.Expr) bool {
	for _, expr := range exprs {
		if isUnknownExprType(expr) {
			return true
		}
	}
	return false
}

func isTypeOneOf(t ir.Type, kinds ...ir.TypeKind) bool {
	for _, kind := range kinds {
		if t.Kind() == kind {
			return true
		}
	}
	return false
}

func isUntypedBody(t ir.Type) bool {
	switch t := ir.ToBaseType(t).(type) {
	case *ir.BasicType:
	case *ir.StructType:
		return !t.TypedBody
	case *ir.ArrayType:
		return isUntypedBody(t.Elem)
	case *ir.SliceType:
		return isUntypedBody(t.Elem)
	case *ir.PointerType:
		return isUntypedBody(t.Elem)
	case *ir.FuncType:
		for _, param := range t.Params {
			if isUntypedBody(param.T) {
				return true
			}
		}
	}
	return false
}

func isIncompleteType(t ir.Type, outer ir.Type) bool {
	incomplete := false
	switch t := ir.ToBaseType(t).(type) {
	case *ir.BasicType:
		if t.Kind() == ir.TVoid {
			if outer == nil || outer.Kind() != ir.TPointer {
				incomplete = true
			}
		}
	case *ir.StructType:
		if t.Opaque() {
			if outer == nil || outer.Kind() != ir.TPointer {
				incomplete = true
			}
		}
	case *ir.ArrayType:
		incomplete = isIncompleteType(t.Elem, t)
	case *ir.SliceType:
		if !t.Ptr {
			incomplete = true
		} else {
			incomplete = isIncompleteType(t.Elem, t)
		}
	case *ir.PointerType:
		incomplete = isIncompleteType(t.Elem, t)
	}
	return incomplete
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
		deref := &ir.DerefExpr{
			X: expr,
		}
		deref.SetRange(expr.Pos(), expr.EndPos())
		deref.T = tres
		return deref
	}

	return expr
}

func ensureCompatibleType(expr ir.Expr, target ir.Type) ir.Expr {
	if promoted, ok := tryPromoteConstType(expr, target); ok {
		return promoted
	}
	if casted, ok := tryImplicitCast(expr, target); ok {
		return casted
	}
	return expr
}

func tryPromoteConstType(expr ir.Expr, target ir.Type) (ir.Expr, bool) {
	texpr := expr.Type()
	promote := false
	if texpr.Kind() == ir.TConstInt {
		if target == nil {
			target = ir.TBuiltinInt
		}
		if ir.IsNumericType(target) {
			promote = true
		}
	} else if texpr.Kind() == ir.TConstFloat {
		if target == nil {
			target = ir.TBuiltinFloat
		}
		if ir.IsFloatType(target) {
			promote = true
		}
	} else if texpr.Kind() == ir.TNull {
		if target == nil {
			target = ir.NewPointerType(ir.TBuiltinInt8, false)
		}
		if isTypeOneOf(target, ir.TPointer, ir.TSlice, ir.TFunc) {
			promote = true
		}
	}
	if promote {
		if lit, ok := expr.(*ir.BasicLit); ok {
			lit.SetType(target)
			return lit, true
		}
		cast := &ir.CastExpr{}
		cast.SetRange(expr.Pos(), expr.EndPos())
		cast.X = expr
		cast.T = target
		return cast, true
	}
	return expr, false
}

func tryImplicitCast(expr ir.Expr, target ir.Type) (ir.Expr, bool) {
	if target == nil {
		return expr, false
	}
	cast := false
	from := ir.ToBaseType(expr.Type())
	to := ir.ToBaseType(target)
	switch from := from.(type) {
	case *ir.BasicType:
		if to, ok := to.(*ir.BasicType); ok {
			if ir.IsIntegerType(from) {
				if ir.IsIntegerType(to) {
					if from.Kind() < to.Kind() {
						if ir.IsSignedType(to) {
							cast = true
						} else if ir.IsUnsignedType(from) {
							cast = true
						}
					}
				}
			} else if ir.IsFloatType(from) {
				if ir.IsFloatType(to) {
					cast = from.Kind() < to.Kind()
				}
			}
		}
	case *ir.SliceType:
		if to, ok := to.(*ir.SliceType); ok {
			if !from.ReadOnly && to.ReadOnly {
				cast = from.Elem.Equals(to.Elem)
			}
		}
	case *ir.PointerType:
		if from.Kind() == ir.TUnknown {
			if isTypeOneOf(to, ir.TPointer, ir.TSlice, ir.TFunc) {
				cast = true
			}
		} else if to, ok := to.(*ir.PointerType); ok {
			if !from.ReadOnly && to.ReadOnly {
				cast = (to.Elem.Kind() == ir.TVoid) || (from.Elem.Equals(to.Elem))
			} else if from.ReadOnly == to.ReadOnly {
				cast = (to.Elem.Kind() == ir.TVoid) && (from.Elem.Kind() != ir.TVoid)
			}
		}
	}
	if cast {
		cast := &ir.CastExpr{X: expr}
		cast.SetRange(expr.Pos(), expr.EndPos())
		cast.T = target
		return cast, true
	}
	switch to := to.(type) {
	case *ir.PointerType:
		if to.ReadOnly && to.Elem.Equals(from) {
			addr := &ir.AddrExpr{
				X:         expr,
				Immutable: true,
			}
			addr.SetRange(expr.Pos(), expr.EndPos())
			addr.T = target
			return addr, true
		}
	}
	return expr, false
}
