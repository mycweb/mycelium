# Mycelium

Mycelium is a set of typed formats for storing and transferring data.
As you might expect it supports things like:
- Bits
- Arrays
- [Products](https://en.wikipedia.org/wiki/Product_(category_theory))
- Sums ([Coproducts](https://en.wikipedia.org/wiki/Coproduct))
- Lists

But as you might not expect, it also has support for:
- Refs (pointers) 
- Expressions
- Functions/Procedures
- Types

Every value that you can specify in Mycelium has a Type (which is also a Value), and every Value can be stored or transferred.
Values can be computed by evaluating Expressions.
Expressions are also Values and can also be stored or transferred.

## Virtual Machine `mvm1/`
The Mycelium Virtual Machine (MVM) is an abstract machine model that defines what it means for a Mycelium Expression to be evaluated.
It is not necessary to have an MVM implementation in order to send and receive Mycelium Values.
A reference implementation of the MVM, written in Go, is available in the `mvm1` directory.

Here are some fast facts about the MVM:
- All Values in the MVM are immutable.
- The MVM models interprocess interactions as operations {Input, Output, Interact} on special Values called *Ports*.  These are the only operations with effects.
- Since all Values can be fingerprinted in constant time, it is very easy to add accelerators for known functions. e.g. the MVM has no addition operation, but a cannonical addition function is accelerated using the hardware's addition instruction.
- The MVM operations are defined in [`spec/op.go`](./spec/op.go).  The design favors a small amount of powerful operations over a large amount.


## MycZip `myczip/`
MycZip is a format for encoding Mycelium values so they may exist in a filesystem as a single file.
A single MycZip file can hold a single Mycelium Value.
This is hardly a limitation, since that Value can be an Array or Product, and transitively include as many other Values as desired.


## Network Protocol `mycnet/`
The Mycelium Network Protocol (MNP) is a protocol for transferring Mycelium Values over the network between Peers.

Built on [QUIC](https://en.wikipedia.org/wiki/QUIC), it is a peer-to-peer message passing protocol, with support for Ask and Tell semantics on messages containing Mycelium Values.
All parties are authenticated in MNP, and the cryptographic identity of each party has a cannonical representation as a Mycelium Value (called a Peer).

MNP caches all transferred values, and only Values not available locally will be transferred. e.g. If you send a lot of strings, you will only have to send the type for a String (cannonically `List[Array[Bit, 8]]`) the first time, and then never again.
