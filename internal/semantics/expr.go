package semantics

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

func (c *checker) checkArrayLit(expr *ir.ArrayLit) ir.Expr {
	expr.Elem = c.checkRootTypeExpr(expr.Elem, true)
	tpunt := untypedExpr(expr.Elem, nil)
	if expr.Size != nil {
		expr.Size = c.checkExpr(expr.Size)
		tpunt = untypedExpr(expr.Size, nil)
	}

	for i, init := range expr.Initializers {
		init = c.checkExpr(init)
		expr.Initializers[i] = init
		tpunt = untypedExpr(init, tpunt)
	}

	if tpunt != nil {
		expr.T = tpunt
		return expr
	}

	size := 0

	if expr.Size != nil {
		// TODO: Merge with ArrayTypeExpr
		expr.Size = c.finalizeExpr(expr.Size, nil)
		if !ir.IsIntegerType(expr.Size.Type()) {
			c.error(expr.Size.Pos(), "array size expects an integer type (got %s)", expr.Size.Type())
			expr.Size.SetType(ir.TBuiltinInvalid)
		} else if lit, ok := expr.Size.(*ir.BasicLit); !ok {
			c.error(expr.Size.Pos(), "array size is not a constant expression")
			expr.Size.SetType(ir.TBuiltinInvalid)
		} else if lit.NegatigeInteger() {
			c.error(expr.Size.Pos(), "array size cannot be negative")
			expr.Size.SetType(ir.TBuiltinInvalid)
		} else if lit.Zero() {
			c.error(expr.Size.Pos(), "array size cannot be zero")
			expr.Size.SetType(ir.TBuiltinInvalid)
		} else {
			size = int(lit.AsU64())
		}
		if size == 0 {
			expr.T = ir.TBuiltinInvalid
			return expr
		}
	}

	if size == 0 {
		if len(expr.Initializers) == 0 {
			c.error(expr.Pos(), "array literal must have a size")
			expr.T = ir.TBuiltinInvalid
			return expr
		}
	} else {
		if size != len(expr.Initializers) {
			c.error(expr.Pos(), "array size %d different from number of elements %d", size, len(expr.Initializers))
			expr.T = ir.TBuiltinInvalid
			return expr
		}
	}

	telem := expr.Elem.Type()

	for i, init := range expr.Initializers {
		init = c.finalizeExpr(init, telem)
		expr.Initializers[i] = init
		if isTypeMismatch(telem, init.Type()) {
			c.error(init.Pos(), "type mismatch of array element (expected type %s, got type %s)", telem, init.Type())
			telem = ir.TBuiltinInvalid
			break
		}
	}

	if isUntyped(telem) {
		expr.T = telem
	} else {
		expr.T = ir.NewArrayType(telem, len(expr.Initializers))
	}

	return expr
}

func (c *checker) checkDotExpr(expr *ir.DotExpr) ir.Expr {
	if isUnresolvedExpr(expr.X) {
		expr.X = c.checkExpr2(expr.X, modeExprOrType)
		expr.X = tryDeref(expr.X)
	}

	if tpunt := puntExprs(expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	tx := expr.X.Type()

	switch tx := ir.ToBaseType(tx).(type) {
	case *ir.ModuleType:
		defer c.setScope(c.setScope(tx.Scope))
		return c.checkIdent(expr.Name)
	case *ir.StructType:
		if tx.Opaque() {
			c.nodeError(expr.X, "expression has incomplete type %s", tx)
			expr.T = ir.TBuiltinInvalid
		} else {

			prevScope := c.setScope(tx.Scope)
			if sym, ok := c.resolveIdent(expr.Name); ok {
				expr.Name.Sym = sym
				expr.Name.T = sym.T
				expr.T = sym.T
			} else {
				expr.T = ir.TBuiltinInvalid
			}
			c.setScope(prevScope)
		}
	default:
		c.error(expr.X.Pos(), "expression cannot be used for field access (has type %s)", tx)
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinInvalid
	}

	return expr
}

func (c *checker) checkIndexExpr(expr *ir.IndexExpr) ir.Expr {
	expr.X = c.checkExpr(expr.X)
	expr.X = c.finalizeExpr(expr.X, nil)

	expr.Index = c.checkExpr(expr.Index)
	expr.Index = c.finalizeExpr(expr.Index, nil)

	if tpunt := puntExprs(expr.X, expr.Index); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	expr.X = tryDeref(expr.X)
	var telem ir.Type

	switch tx := ir.ToBaseType(expr.X.Type()).(type) {
	case *ir.ArrayType:
		telem = tx.Elem
	case *ir.SliceType:
		telem = tx.Elem
	}

	if telem != nil {
		tindex := expr.Index.Type()
		if !ir.IsIntegerType(tindex) {
			c.error(expr.Index.Pos(), "index expects an integer type (got %s)", tindex)
		} else {
			expr.T = telem
		}
	} else {
		c.error(expr.X.Pos(), "expression cannot be indexed (has type %s)", expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinInvalid
	}

	return expr
}

func (c *checker) checkSliceExpr(expr *ir.SliceExpr) ir.Expr {
	expr.X = c.checkExpr(expr.X)
	expr.X = c.finalizeExpr(expr.X, nil)

	if expr.Start != nil {
		expr.Start = c.checkExpr(expr.Start)
		expr.Start = c.finalizeExpr(expr.Start, nil)
	}

	if expr.End != nil {
		expr.End = c.checkExpr(expr.End)
		expr.End = c.finalizeExpr(expr.End, nil)
	}

	if tpunt := puntExprs(expr.X, expr.Start, expr.End); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	expr.X = tryDeref(expr.X)

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
			if !ir.IsIntegerType(tstart) {
				c.error(expr.Start.Pos(), "slice index expects an integer type (got %s)", tstart)
				err = true
			}
		}

		if expr.End != nil {
			tend := expr.End.Type()
			if !ir.IsIntegerType(tend) {
				c.error(expr.End.Pos(), "slice index expects an integer type (got %s)", tend)
				err = true
			}
		}

		if !err {
			if expr.Start == nil {
				expr.Start = createIntLit(0, ir.TBuiltinUSize)
			} else if expr.Start.Type().Kind() != ir.TUSize {
				cast := &ir.CastExpr{X: expr.Start}
				cast.SetRange(expr.Start.Pos(), expr.Start.EndPos())
				cast.T = ir.TBuiltinUSize
				expr.Start = cast
			}

			if expr.End == nil {
				if tx.Kind() == ir.TPointer {
					c.nodeError(expr, "end index is required when slicing pointer types (got %s)", tx)
					err = true
				} else {
					len := &ir.LenExpr{
						X: expr.X,
					}
					len.T = ir.TBuiltinUSize
					expr.End = len
				}
			} else if expr.End.Type().Kind() != ir.TUSize {
				cast := &ir.CastExpr{X: expr.End}
				cast.SetRange(expr.End.Pos(), expr.End.EndPos())
				cast.T = ir.TBuiltinUInt64
				expr.End = cast
			}
		}

		if !err {
			expr.T = ir.NewSliceType(telem, ro, false)
		}
	} else {
		c.nodeError(expr.X, "expression cannot be sliced (has type %s)", tx)
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinInvalid
	}

	return expr
}

func (c *checker) checkAppExpr(expr *ir.AppExpr) ir.Expr {
	var tpunt ir.Type
	if isUnresolvedExpr(expr.X) {
		expr.X = c.checkExpr2(expr.X, modeExprOrType)
		tx := expr.X.Type()
		if isUntyped(tx) {
			tpunt = tx
		} else {
			ok := false
			if tx.Kind() == ir.TFunc {
				ok = true
			} else if tx.Kind() == ir.TStruct {
				if sym := ir.ExprSymbol(expr.X); sym != nil {
					if sym.Kind == ir.TypeSymbol {
						ok = true
					}
				}
			}
			if !ok {
				c.nodeError(expr.X, "non-applicable expression (has type %s)", tx)
				expr.T = ir.TBuiltinInvalid
				return expr
			}
		}
	}

	for i, arg := range expr.Args {
		expr.Args[i].Value = c.checkExpr(arg.Value)
		tpunt = untypedExpr(expr.Args[i].Value, tpunt)
	}

	if tpunt != nil {
		expr.T = tpunt
		return expr
	}

	tx := expr.X.Type()

	if tx.Kind() == ir.TFunc {
		tfun := ir.ToBaseType(tx).(*ir.FuncType)
		expr.Args = c.checkArgumentList(tx, expr.Args, tfun.Params, false)
		expr.T = tfun.Return
	} else { // struct
		tstruct := ir.ToBaseType(tx).(*ir.StructType)
		if !tstruct.TypedBody {
			expr.T = ir.TBuiltinUnknown
			return expr
		}
		expr.Args = c.checkArgumentList(tstruct, expr.Args, tstruct.Fields, true)
		expr.T = tx
		expr.IsStruct = true
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
					c.nodeError(arg, "unknown named argument '%s'", arg.Name.Literal)
				}
			} else if named {
				c.nodeError(arg, "positional argument after named argument is not allowed")
				mixed = true
			} else if argIndex < len(fields) {
				fieldIndex = argIndex
			} else {
				break
			}

			if fieldIndex >= 0 {
				field := fields[fieldIndex]
				if existing := argsRes[fieldIndex]; existing != nil {
					// This is only possible if the current argument is named
					c.nodeError(arg, "duplicate arguments for '%s' at position %d", arg.Name.Literal, fieldIndex+1)
				} else {
					arg.Value = c.finalizeExpr(arg.Value, field.T)
					if isTypeMismatch(arg.Value.Type(), field.T) {
						kind := "parameter"
						if tobj.Kind() == ir.TStruct {
							kind = "field"
						}
						if arg.Name != nil {
							c.nodeError(arg, "%s '%s' at position %d expects type %s (got %s)", kind, arg.Name.Literal, fieldIndex+1, field.T, arg.Value.Type())
						} else {
							c.nodeError(arg, "%s at position %d expects type %s (got %s)", kind, fieldIndex+1, field.T, arg.Value.Type())
						}
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
					arg.Value = ir.NewDefaultInit(field.T)
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
	expr.ToType = c.checkRootTypeExpr(expr.ToType, true)
	expr.X = c.checkExpr(expr.X)

	if tpunt := puntExprs(expr.ToType, expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	sym := ir.ExprSymbol(expr.ToType)
	if sym != nil && sym.Kind != ir.TypeSymbol {
		c.error(expr.ToType.Pos(), "'%s' is not a type", sym.Name)
	} else {
		t1 := expr.ToType.Type()
		t2 := expr.X.Type()

		if t2.CastableTo(t1) {
			expr.T = t1
		} else {
			c.error(expr.X.Pos(), "type %s cannot be cast to %s", t2, t1)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinInvalid
	}

	return expr
}

func (c *checker) checkLenExpr(expr *ir.LenExpr) ir.Expr {
	if isUntypedExpr(expr.X) {
		expr.X = c.checkExpr(expr.X)
		expr.X = tryDeref(expr.X)
	}
	if tpunt := puntExprs(expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}
	switch tx := ir.ToBaseType(expr.X.Type()).(type) {
	case *ir.ArrayType:
		expr.T = ir.TBuiltinUSize
	case *ir.SliceType:
		expr.T = ir.TBuiltinUSize
	default:
		c.error(expr.X.Pos(), "type %s does not have a length", tx)
		expr.T = ir.TBuiltinInvalid
	}
	return expr
}

func (c *checker) checkSizeExpr(expr *ir.SizeExpr) ir.Expr {
	expr.X = c.checkRootTypeExpr(expr.X, true)
	if tpunt := puntExprs(expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}
	tx := expr.X.Type()
	if isUntypedBody(tx) {
		expr.T = ir.TBuiltinUnknown
		return expr
	}
	size := c.target.Sizeof(tx)
	return createIntLit(size, ir.TBuiltinUSize)
}

func createIntLit(val int, t ir.Type) ir.Expr {
	lit := &ir.BasicLit{Tok: token.Integer, Value: strconv.FormatInt(int64(val), 10)}
	lit.T = t
	if val == 0 {
		lit.Raw = ir.BigIntZero
	} else {
		lit.Raw = big.NewInt(int64(val))
	}
	return lit
}

func (c *checker) checkBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	expr.Left = c.checkExpr(expr.Left)
	expr.Right = c.checkExpr(expr.Right)

	if tpunt := puntExprs(expr.Left, expr.Right); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	expr.Left = ensureCompatibleType(expr.Left, expr.Right.Type())
	expr.Right = ensureCompatibleType(expr.Right, expr.Left.Type())

	tleft := expr.Left.Type()
	tright := expr.Right.Type()

	if !tleft.Equals(tright) {
		c.nodeError(expr, "type mismatch %s and %s", tleft, tright)
		expr.T = ir.TBuiltinInvalid
		return expr
	}

	badop := false
	logicop := expr.Op.OneOf(token.Land, token.Lor)
	eqop := expr.Op.OneOf(token.Eq, token.Neq)
	orderop := expr.Op.OneOf(token.Gt, token.GtEq, token.Lt, token.LtEq)
	mathop := expr.Op.OneOf(token.Add, token.Sub, token.Mul, token.Div, token.Mod)

	toperand := expr.Left.Type()
	texpr := toperand
	if logicop || eqop || orderop {
		texpr = ir.TBuiltinBool
	}

	if ir.IsNumericType(toperand) {
		if logicop {
			badop = true
		}
	} else if toperand.Kind() == ir.TBool {
		if orderop || mathop {
			badop = true
		}
	} else if toperand.Kind() == ir.TPointer {
		if orderop || mathop {
			badop = true
		}
	} else {
		badop = true
	}

	if badop {
		c.nodeError(expr, "operator '%s' cannot be performed on types %s and %s", expr.Op, expr.Left.Type(), expr.Right.Type())
		texpr = ir.TBuiltinInvalid
	}

	expr.T = texpr

	return expr
}

func (c *checker) checkUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	expr.X = c.checkExpr(expr.X)
	tx := expr.X.Type()

	if tpunt := puntExprs(expr.X); tpunt != nil {
		expr.T = tpunt
		return expr
	}

	switch expr.Op {
	case token.Sub:
		if ir.IsNumericType(tx) {
			expr.T = tx
		} else {
			c.error(expr.Pos(), "additive inverse cannot be performed on type %s", tx)
		}
	case token.Lnot:
		if tx.Kind() == ir.TBool {
			expr.T = tx
		} else {
			c.error(expr.Pos(), "logical not cannot be performed on type %s)", tx)

		}
	case token.Deref:
		switch t := ir.ToBaseType(tx).(type) {
		case *ir.PointerType:
			expr.T = t.Elem
		default:
			c.error(expr.X.Pos(), "expression cannot be dereferenced (has type %s)", tx)
		}
	case token.Addr:
		ro := expr.Decl.Is(token.Val)
		if !expr.X.Lvalue() {
			c.error(expr.X.Pos(), "expression is not an lvalue")
		} else if expr.X.ReadOnly() && !ro {
			c.error(expr.X.Pos(), "expression is read-only")
		} else {
			if tslice, ok := tx.(*ir.SliceType); ok {
				if !tslice.Ptr {
					tslice.Ptr = true
					tslice.ReadOnly = ro
					expr.T = tslice
				} else {
					expr.T = ir.NewPointerType(tslice, ro)
				}
			} else {
				expr.T = ir.NewPointerType(tx, ro)
			}
		}
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op))
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinInvalid
	}

	return expr
}

// TODO: Move to lexer and add better support for escape sequences.
func (c *checker) unescapeStringLiteral(lit *ir.BasicLit) (string, bool) {
	escaped := []rune(lit.Value)
	var unescaped []rune

	start := 0
	n := len(escaped)

	// Remove quotes
	if n >= 2 {
		start++
		n--
	}

	for i := start; i < n; i++ {
		ch1 := escaped[i]
		if ch1 == '\\' && (i+1) < len(escaped) {
			i++
			ch2 := escaped[i]

			if ch2 == 'a' {
				ch1 = 0x07
			} else if ch2 == 'b' {
				ch1 = 0x08
			} else if ch2 == 'f' {
				ch1 = 0x0c
			} else if ch2 == 'n' {
				ch1 = 0x0a
			} else if ch2 == 'r' {
				ch1 = 0x0d
			} else if ch2 == 't' {
				ch1 = 0x09
			} else if ch2 == 'v' {
				ch1 = 0x0b
			} else if ch2 == '"' {
				ch1 = ch2
			} else {
				c.error(lit.Pos(), "invalid escape sequence '\\%c'", ch2)
				return "", false
			}
		}
		unescaped = append(unescaped, ch1)
	}

	return string(unescaped), true
}

func removeUnderscores(lit string) string {
	res := strings.Replace(lit, "_", "", -1)
	return res
}

func (c *checker) checkBasicLit(expr *ir.BasicLit) ir.Expr {
	if expr.Tok == token.False || expr.Tok == token.True {
		expr.T = ir.TBuiltinBool
	} else if expr.Tok == token.Char {
		if raw, ok := c.unescapeStringLiteral(expr); ok {
			val := big.NewInt(0)
			val.SetUint64(uint64(raw[0]))
			expr.Raw = val
			expr.T = ir.NewBasicType(ir.TConstInt)
		} else {
			expr.T = ir.TBuiltinInvalid
		}
	} else if expr.Tok == token.String {
		if expr.Raw == nil {
			if raw, ok := c.unescapeStringLiteral(expr); ok {
				if expr.Prefix == nil {
					expr.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
					expr.Raw = raw
				} else {
					prefix := ir.ExprToFQN(expr.Prefix)
					if prefix == "c" {
						expr.T = ir.NewPointerType(ir.TBuiltinInt8, true)
						expr.Raw = raw
					} else {
						c.error(expr.Prefix.Pos(), "invalid string prefix '%s'", prefix)
						expr.T = ir.TBuiltinInvalid
					}
				}
			} else {
				expr.T = ir.TBuiltinInvalid
			}
		}
	} else if expr.Tok == token.Integer {
		if expr.Raw == nil {
			target := ir.TBuiltinConstInt

			if expr.Suffix != nil {
				suffix := ir.ExprToFQN(expr.Suffix)
				switch suffix {
				case ir.TFloat64.String():
					target = ir.TBuiltinFloat64
				case ir.TFloat32.String():
					target = ir.TBuiltinFloat32
				case ir.TUSize.String():
					target = ir.TBuiltinUSize
				case ir.TUInt64.String():
					target = ir.TBuiltinUInt64
				case ir.TUInt32.String():
					target = ir.TBuiltinUInt32
				case ir.TUInt16.String():
					target = ir.TBuiltinUInt16
				case ir.TUInt8.String():
					target = ir.TBuiltinUInt8
				case ir.TInt64.String():
					target = ir.TBuiltinInt64
				case ir.TInt32.String():
					target = ir.TBuiltinInt32
				case ir.TInt16.String():
					target = ir.TBuiltinInt16
				case ir.TInt8.String():
					target = ir.TBuiltinInt8
				default:
					c.error(expr.Suffix.Pos(), "invalid int suffix '%s'", suffix)
					target = ir.TBuiltinInvalid
				}
			}

			if !isUntyped(target) {
				normalized := removeUnderscores(expr.Value)
				if ir.IsIntegerType(target) {
					val := big.NewInt(0)
					_, ok := val.SetString(normalized, 0)
					if ok {
						expr.Raw = val
					}
				} else if ir.IsFloatType(target) {
					val := big.NewFloat(0)
					_, ok := val.SetString(normalized)
					if ok {
						expr.Raw = val
					}
				}
				if expr.Raw != nil {
					expr.T = target
				} else {
					c.error(expr.Pos(), "unable to interpret int literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinInvalid
			}
		}
	} else if expr.Tok == token.Float {
		if expr.Raw == nil {
			target := ir.TBuiltinConstFloat

			if expr.Suffix != nil {
				suffix := ir.ExprToFQN(expr.Suffix)
				switch suffix {
				case ir.TFloat64.String():
					target = ir.TBuiltinFloat64
				case ir.TFloat32.String():
					target = ir.TBuiltinFloat32
				default:
					c.error(expr.Suffix.Pos(), "invalid float suffix '%s'", suffix)
					target = ir.TBuiltinInvalid
				}
			}

			if !isUntyped(target) {
				val := big.NewFloat(0)
				normalized := removeUnderscores(expr.Value)
				_, ok := val.SetString(normalized)
				if ok {
					expr.T = target
					expr.Raw = val
				} else {
					c.error(expr.Pos(), "unable to interpret float literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinInvalid
			}
		}
	} else if expr.Tok == token.Null {
		expr.T = ir.NewBasicType(ir.TNull)
	} else {
		panic(fmt.Sprintf("Unhandled literal %s at %s", expr.Tok, expr.Pos()))
	}

	return expr
}
