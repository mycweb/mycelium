;; Cell is an interface for cell types
(defc Cell (Ref (Product
    (Lambda Unit Any) ;; Load
    (Lambda (Product Any) Unit) ;; Put
    (Lambda (Product Any Any) Any) ;; CAS
)))

(defl load {cell: Cell} Any
    ((!field (!load cell) 0) {})
)

(defl put {cell: Cell, x: Any} Unit
    ((!field (!load cell) 1) x)
)

(defl cas {cell: Cell, prev: Any, next: Any} Any
    ((!field (!load cell) 2) prev next)
)

(defl apply {cell: Cell, fn: (Lambda (Product AnyValue) AnyValue)} Unit
)

(pub Cell load put cas)