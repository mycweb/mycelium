# Operations

> The notation in this section uses `=> y` to mean "evaluates to y" and `-> Y` to mean "evaluates to a value of type Y".

> An argument with an `'` (single quote) means it is not evaluated before the operation.

## General & Craft/Uncraft

### `pass(x) => x`
Pass is a no-op, it evalutes to it's first argument.

### `equal(a: Value, b Value) -> Bit`
Equal evaluates to 1 if a and b are equal, and 0 if not.
This comparision can always be done in constant time.

### `craft(T: Type, x: _) -> T`
Craft is used to create a value of a given type using an equivalent bit pattern.
Before and after a craft operation, none of the bits need to change.  The operation is for type checking the computation.
The output type of Craft is always the same as its first argument.

Below are the results of using craft with a *Kind*
```
craft(BitKind, {}) => BitType
craft(ArrayKind, {T: Type, l: Size}) => Array[T, l]
craft(RefKind, {T: Type}) => Ref[T]
craft(ProgKind, {l: Size}) => ProgType[l]
craft(SumKind, Array[l, AnyType]) -> SumType[l]
craft(ProductKind, Array[l, AnyType]) ->
craft(ListKind, a Type) => List[a]
craft(LazyKind, a Type) => Lazy[a]
craft(LambdaKind, a, b) => Lambda[a, b]
```

### `uncraft(x: X) -> Y`
Uncraft reverses Craft by returning the second argument.
The following relationship always holds.
```
craft(typeOf(x), uncraft(x)) = x
uncraft(craft(typeOf(x), x)) = x
```

```
uncraft(ArrayType) -> Product[Type, Size]
uncraft(ProgType) -> Product[Size]
uncraft(RefType) -> Product[Type]
uncraft(SumType) -> Array[Type, _]
uncraft(ProductType) -> Array[Type, _]
uncraft(ListType) -> Product[Type]
uncraft(LazyType) -> Prouduct[Type]
uncraft(LambdaType) -> Product[Type, Type]
uncraft(PortType) -> Product[Type, Type, Type, Type]
uncraft(DistinctType) -> Product[Type, Value]
```

### `typeOf(x: 'T) => T`
TypeOf returns the type of an expression without evaluating it

### `sizeOf(x: Type) -> Size`
SizeOf returns the size of values of a Type x.  It is not the size of x itself.

## Compute

### `let(x, body)`
Binds an additional parameter in the context with the value of `x`.
Then let evaluates `body` in the new context with the additional bound parameter.

### `lazy(out: Type, body: AnyProg) -> Lazy[out]`
Creates a lazy value from body.
Body is not evaluated.
Any parameters in body will be evaluated at the time lazy is called.
If body, given its bound parameters does evaluate to type out, then a TypeError fault occurs.

### `lambda(in: Type, out: Type, body: AnyProg) -> Lambda[in, out]`
Creates a lambda value from body.
Body is not evaluated.
Parameters > 0 will be evaluated at the time lambda is called.
Parameter 0 (written as `%0`) is assumed to be of type `in`.
If body does not evaluated to type `out` then a TypeError fault occurs.

### `fractal(body: AnyProg) -> FractalType`
Creates a Fractal type defined by the body expression, which *must* contain a self reference.

### `eval(x: Lazy[T]) -> T`
Evaluates a Lazy Value.
Eval has a side effect on the Lazy Value, future calls to Eval will not re-evaluate the expression, and will instead return a cached value.

### `apply(fn: Lambda[In, Out], x: In) -> Out`
Binds x to `%0` and evaluates the body of fn in that context.

## Composite
Composite values are made of other values.

### `slot`
Slot is used to access an element of an Array or List

### `field`
Field is used to access an element of a Product or Sum.

```
field(x: Product[A, B, C], idx: 0) -> A
field(x: Product[A, B, C], idx: 1) -> B
field(x: Product[A, B, C], idx: 2) -> C

field(x: Sum[A, B, C], idx: 1) -> A
field(x: Sum[A, B, C], idx: 1) -> B
field(x: Sum[A, B, C], idx: 1) -> C
```

### `which(x: Sum[...]) -> Size`
Which returns the tag of a Sum padded to fill a Size Value.

### `len(x) -> Size`
The index passed to get must always be less than the Size returned by len

### `concat`
Concat is defined on Arrays, Lists, and Products.

```
concat(left: Array[T, l1], right: Array[T, l2]) -> Array[T, l1 + l2]
concat(left: List[T], right: List[T] -> List[T])
concat(left: Product[L0 ... Ln], Product[R0, Rn]) -> Product[L0 ... Ln, R0 ... Rn]
```

## Array Compute

### `map`
```
map(xs: Array[T, _], fn: Lambda[Product[T, T], T]) T
map(xs: List[T], fn: Lambda[Product[T, T], T]) T
```

### `zip`
```
zip(as: Array[A, l], bs: Array[B, l]) Array[Product[A, B], l]
zip(as: List[A], bs: List[B]) List[Product[A, B]]
```

### `reduce`
```
reduce(xs: List[T], fn: Lambda[Product[T, T], T]) T
reduce(xs: Array[T, _], fn: Lambda[Product[T, T], T]) T
```

Reduce implements a binary reduce tree on Arrays and Lists for associative, non-commutative functions.

### `fold`
Fold implements a fold-left over an array or list

### `reshape(ty: Type, x: _) ty`
Reshape converts a value containing only Bits into a value of another type also containing only bits.
The current type and the new type must have values of the same size.
Reshape requires a type known at compile time.
