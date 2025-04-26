;; package spore provides an Abstract Syntax Tree for the Spore language.

;; Number is a terminal node in the AST containing integers
(defc Number (Distinct (List Bit) "spore/ast.Number"))
;; String is a terminal node in the AST containing unicode data
(defc String (Distinct (List Byte) "spore/ast.String"))
;; Param is a terminal node in the AST representing a parameter available at runtime
(defc Param (Distinct (Array Bit 32) "spore/ast.Param"))
;; Op is a terminal node in the AST representing a primitive operation
(defc Op (Distinct (List Byte) "spore/ast.Op"))
;; Symbol is a a terminal node in the AST representing another AST in the context by reference
(defc Symbol (Distinct (List Byte) "spore/ast.Symbol"))

;; Node is a node in the AST
(defc Node (Fractal (Sum
    Number
    String
    Param
    Symbol
    Op
    (Distinct (List (self)) "spore/ast.Expr")
    (Distinct (List (self)) "spore/ast.Tuple")
    (Distinct (List (self)) "spore/ast.Array")
    (Distinct (List (Product (self) (self))) "spore/ast.Table")
)))

;; Expr is a tree node in the AST
(defc Expr (Distinct (List Node) "spore/ast.Expr"))
;; Tuple is a tree node in the AST
(defc Tuple (Distinct (List Node) "spore/ast.Tuple"))
;; Array is a tree node in the AST
(defc Array (Distinct (List Node) "spore/ast.Array"))
;; Table is a tree node in the AST
(defc Row (Product Node Node))
(defc Table (Distinct (List Row) "spore/ast.Table"))

(pub
    Number String Param Symbol Op
    Expr Tuple Array Table
    Node
)

(defc Macro (Distinct (Lambda (List Node) Node) "spore/ast.Macro"))

(defl mkMacro {fn: (Lambda Node Node)} Macro
    (distinct Macro fn)
)

(pub Macro mkMacro)