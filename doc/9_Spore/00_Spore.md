# Spore
Spore is a programming language which compiles to Mycelium Values.

Spore cosmentically resembles a LISP.
It emphasizes easily defining Mycelium Values in a human readable way.

## Numbers
Natural numbers can be written in the usual way.
e.g. `12345`

Leading 0s are allowed.
e.g. `000123`

Underscores do not impact the value of the number, and are useful for readability.
e.g. `123_456_789`

Numbers are assumed to be in base 10 (decimal) by default.
Numbers can also be specified in binary, octal, and hexidecimal in addition to decimal. e.g.
`0b0110`
`0o6440`
`0xdeadbeef`

## Strings
Strings start and end with double quotes `"`.
e.g. `"hello world"`
Strings compile to Lists of Bytes.

```
(defc Byte (Array Bit 8))
(defc String (List Byte))
```

## Parameters
Parameters start with a `%` and are immediately followed by a number e.g.
`%1`.
Parameters are used to refer to values in the runtime context which are not known at compile time.

## Primitive Operations
A primitive operation is something that Mycelium provides as an operation.  Things like `Concat` or `Lambda`.
primitive operations start with an `!`. e.g.
`!concat`
`!lambda`

Primitives are only valid as the first element of an Expr.
`(!concat left right)`

Primitives are available in all contexts, independent of which symbols have been defined.
This is one way that Spore differs from most LISPs, which may dilliberately obscure the substrate to maintain the illusion of symbols and S-Expressions all the way down forever.
It is not possible to define a symbol that begins with a `!`, so Symbols can be disambiguated from Primitive Operations.

## Symbols
Symbols are used to refer to another expression without repeating it.
Symbols must start with a letter or number, but can also contain symbols after the first character. e.g. `abcABC123!@#$%^&*-_<>?`

## Exprs `( ... )`
Exprs--short for expression--are how computation is expressed in Spore.
The first element of the Expr can be a Primitive Operator or another Expr, which resolves to a Lambda or Lazy.


## Array `[ ... ]`
Arrays contain expressions which all evaluate to values of the same type.
Arrays are delineated with square brackets `[` and `]`.


## Tuple `{ ... }`
Tuples contain expressions which can evaluate to values of any type.
Tuples are delineated with curly braces `{` and `}`.

## Table `{ _ : _ , _ : _ ,  ... }`
Tables are composed of Rows.
Rows are defined using `:`.
Each Row has a key and a value.
The key is to the left of the `:` and the value is to the right.
e.g. `key : value`.

Rows are only allowed in a Table, and will fail to parse when specified in other contexts.
