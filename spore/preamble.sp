(package preamble)

;; Kinds
(defc KindKind (!kind 0))
(defc BitKind (!kind 1))
(defc ArrayKind (!kind 2))
(defc ExprKind (!kind 3))
(defc RefKind (!kind 4))
(defc ListKind (!kind 7))
(defc LazyKind (!kind 8))
(defc LambdaKind (!kind 9))
(defc FractalKind (!kind 10))
(defc PortKind (!kind 11))
(defc DistinctKind (!kind 12))

(defc AnyTypeKind (!kind 14))
(defc AnyValueKind (!kind 15))

;; Types
(defc Type (!craft AnyTypeKind {}))
(defc Any (!craft AnyValueKind {}))
(defc Bottom (Sum))
(defc Size (!typeOf (!sizeOf (!anyTypeFrom ()))))
(defc Bit (!craft BitKind {}))

(defc Unit (Product))
(defc Byte (Array Bit 8))
(defc String (List Byte))

;; Boolean
(defc Boolean (Distinct Bit "Boolean"))
(defc true (distinct Boolean 1))
(defc false (distinct Boolean 0))

(defm newList a (!listFrom a))