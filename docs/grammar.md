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

## Declarations

```
ModuleBody      ::= (Module | Include | TopLevelDecl)*
Module          ::= 'module' Name '{' ModuleBody '}'
Include         ::= 'Include' STRING EOS
ScopeName       ::= ('mself' | 'msuper' {'.' 'msuper'}) ['.' Name] | '.'? Name
Name            ::= IDENT ['.' Name]
EOS             ::= ';' | EOF

TopLevelDecl    ::= [Visibility? (ExternDecl | ImportDecl | StructDecl | FuncDecl | Decl)] EOS
Visibility      ::= 'pub' | 'priv'
Extern          ::= 'extern' ['(' IDENT ')']
ExternDecl      ::= Extern (ValDecl | FuncDecl)
StructDecl      ::= 'struct' IDENT StructBody?
FuncDecl        ::= 'fun' IDENT FuncSignature Block?
Decl            ::= TypeDecl | ValDecl | ImportDecl
TypeDecl        ::= 'typealias' IDENT '=' Type
ValDecl         ::= ('val' | 'var') IDENT [':' Type] ['=' Expr]

ImportDecl      ::= ('import' | 'importlocal') ImportName [':' ('('ImportList')' | ImportList) ]
ImportName      ::= [('_' | IDENT) '='] ScopeName
ImportList      ::= ImportItem {',' ImportItem} ','?
ImportItem      ::= [('_' | IDENT) '='] IDENT

Field           ::= (['val' | 'var'] IDENT ':')? Type
StructBody      ::= '{' {Field ';'} '}'
FuncSignature   ::= '(' [Field {',' Field} ','?] ')' Type?
```

## Types

```
Type            ::= NestedType | PointerType | ArrayType | FuncType | ScopeName
NestedType      ::= '(' Type ')'
PointerType     ::= '&' ['val' | 'var'] Type
ArrayType       ::= '[' Type [':' INTEGER] ']'
FuncType        ::= Extern? 'fun' ['[' IDENT ']'] FuncSignature
```

## Statements

```
Block           ::= '{' Stmt* '}'
Stmt            ::= [Block | Decl | ExprStmt | IfStmt | WhileStmt |
                     ForStmt | ReturnStmt | DeferStmt | BranchStmt ] EOS
ExprStmt        ::= Expr ['++' | '--' | (('=' | '+=' | '-=' | '*=' | '/=' | '%=' ) Expr)]
IfStmt          ::= 'if' IfStmt1
IfStmt1         ::=  Expr Block [('elif' IfStmt1) | ('else' Block)]
WhileStmt       ::= 'while' Expr ':' Block
ForStmt         ::= 'for' [IDENT [':' Type] '=' Expr] ';' Expr? ';' ExprStmt? ':' Block
ReturnStmt      ::= 'return' Expr?
BranchStmt      ::= 'break' | 'continue'
DeferStmt       ::= 'defer' ExprOrAssignStmt
```

## Expressions

```
Expr            ::= UnaryOp? Operand  Primary AsExpr [BinaryOp Expr]
BinaryOp        ::= 'or | 'and' | '!=' | '==' | '>' | '>=' | '<' | '<='
                    | '-' | '+' | '/' | '%' | '*'
UnaryOp         ::= ('not' | '-' | '*') | ('&' ['val' | 'var'])
AsExpr          ::= ['as' Type]
Operand         ::= NestedExpr | LenExpr | SizeExpr | ScopeName | Literal
NestedExpr      ::= '(' Expr ')'
LenExpr         ::= 'len' '(' Expr ')'
SizeExpr        ::= 'sizeof' '(' Type ')'

ArgExpr         ::= [IDENT ':'] Expr
ArgList         ::= [ArgExpr {',' ArgExpr} ','?]

Primary         ::= [SliceExpr | IndexExpr | FuncCall | DotExpr]
SliceExpr       ::= '[' Expr? ':' Expr? ']'
IndexExpr       ::= '[' Expr ']' Primary
AppExpr         ::= '(' ArgList )' Primary
DotExpr         ::= '.' IDENT Primary
```

## Literals

```
Literal         ::= BasicLit | ArrayLit | FuncLit
BasicLit        ::= Number | CHAR | (Name? STRING) | 'true' | 'false' | 'null'
Number          ::= (INTEGER | FLOAT) Name?
ArrayLit        ::= ArrayType '{' [Expr {',' Expr} ','?] '}'
FuncLit         ::= Extern? 'fun' FuncSignature Block
```
