# Mycelium Data Types

Mycelium is first and foremost a data serialization format.

There are many serialization formats, here is what makes Mycelium special:
  - Serializes typed data, with full type erasure.
    Sender and receiver must both have the type.
  - Allows references to data, data does not have to be flattened to be serialized.
  - Supports sending functions
  - All Values can be fingerprinted in constant time.  Equal values will always have the same Fingerprint.

In Mycelium all Values have a *Type*, the *Type* determines the meaning of a given bit pattern.
Here are a few key facts about Mycelium's type system.
- The type of a *Type* is a *Kind*.
- The type of a *Kind* is also a *Kind*
- All *Kinds* are *Types*, but not all *Types* are *Kinds*.
- All *Types* are *Values*, but not all *Values* are *Types*.


There are only a few Kinds, each section in this Chapter will describe a different *Kind*.

Mycelium Values have a modest maximum size, this is a [System](./70_System.md) parameter, with `2^24 bits` used in the reference implementations.
