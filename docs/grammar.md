# Grammar

## Notation

```
[..]  : 0 or 1
{..}  : 0 or more
a?    : 0 or 1 a
a*    : 0 or more a
a+    : 1 or more a
a | b : a or b
```

## Entry

```
Entry           ::= ModuleBody
```

## Declarations

```
ModuleBody      ::= {Include | TopDecl}
ScopeLookup     ::= '::'? ScopedName
ScopedName      ::= Ident {'::' Ident}
Include         ::= 'Include' STRING End
End             ::= ';' | EOF

TopDecl         ::= [Visibility? (Module | ImportDecl | ExternDecl | StructDecl | FuncDecl | Decl)] End
Visibility      ::= 'pub' | 'priv'
Module          ::= 'module' ScopedName '{' ModuleBody '}'
ImportDecl      ::= 'import' Alias? ScopedName
Extern          ::= 'extern' ['(' IDENT ')']
ExternDecl      ::= Extern (ValDecl | FuncDecl)
StructDecl      ::= 'struct' IDENT StructBody?
FuncDecl        ::= 'fun' IDENT FuncSignature Block?
Decl            ::= TypeDecl | ValDecl | UseDecl
TypeDecl        ::= 'typealias' IDENT '=' Type
ValDecl         ::= ('val' | 'var') IDENT [':' Type] ['=' Expr]
UseDecl         ::= 'use' Alias? ScopeLookup
Alias           ::= IDENT '='

StructBody      ::= '{' {StructField ';'} '}'
StructField     ::= Visibility? (Field | FuncDecl)
FuncSignature   ::= '(' [Field {',' Field} ','?] ')' Type?
Field           ::= (['val' | 'var'] IDENT ':')? Type
```

## Types

```
Type            ::= NestedType | PointerType | ArrayType | FuncType | ScopeLookup
NestedType      ::= '(' Type ')'
PointerType     ::= '&' ['val' | 'var'] Type
ArrayType       ::= '[' Type [':' INTEGER] ']'
FuncType        ::= Extern? 'fun' ['[' IDENT ']'] FuncSignature
```

## Statements

```
Block           ::= '{' Stmt* '}'
Stmt            ::= [Block | Decl | ExprStmt | IfStmt | WhileStmt |
                     ForStmt | ReturnStmt | DeferStmt | BranchStmt ] End
ExprStmt        ::= Expr ['++' | '--' | (('=' | '+=' | '-=' | '*=' | '/=' | '%=' ) Expr)]
IfStmt          ::= 'if' IfStmt1
IfStmt1         ::=  Expr Block [('elif' IfStmt1) | ('else' Block)]
WhileStmt       ::= 'while' Expr ':' Block
ForStmt         ::= 'for' [IDENT [':' Type] '=' Expr] ';' Expr? ';' ExprStmt? ':' Block
ReturnStmt      ::= 'return' Expr?
BranchStmt      ::= 'break' | 'continue'
DeferStmt       ::= 'defer' ExprStmt
```

## Expressions

```
Expr            ::= UnaryOp? Operand  Primary AsExpr [BinaryOp Expr]
BinaryOp        ::= 'or | 'and' | '!=' | '==' | '>' | '>=' | '<' | '<='
                    | '-' | '+' | '/' | '%' | '*'
UnaryOp         ::= ('not' | '-') | ('&' ['val' | 'var'])
AsExpr          ::= ['as' Type]
Operand         ::= NestedExpr | LenExpr | SizeExpr | ScopeLookup | Literal
NestedExpr      ::= '(' Expr ')'
LenExpr         ::= 'len' '(' Expr ')'
SizeExpr        ::= 'sizeof' '(' Type ')'

ArgList         ::= [ArgExpr {',' ArgExpr} ','?]
ArgExpr         ::= [IDENT ':'] Expr

Primary         ::= [DerefExpr | IndexExpr | SliceExpr | DotExpr | AppExpr]
DerefExpr       ::= '[' ']' Primary
IndexExpr       ::= '[' Expr ']' Primary
SliceExpr       ::= '[' Expr? ':' Expr? ']'
DotExpr         ::= '.' IDENT Primary
AppExpr         ::= '(' ArgList )' Primary
```

## Literals

```
Literal         ::= BasicLit | ArrayLit | FuncLit
BasicLit        ::= Number | CHAR | (IDENT? STRING) | 'true' | 'false' | 'null'
Number          ::= (INTEGER | FLOAT) IDENT?
ArrayLit        ::= ArrayType '(' [Expr {',' Expr} ','?] ')'
FuncLit         ::= Extern? 'fun' FuncSignature Block
```
