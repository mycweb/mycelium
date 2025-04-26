# System

A Mycelium System is a connected network of Mycelium Values, stored and evaluated by substrates.

## Parameters
The previous sections mentioned several parameters which can be varied, but are constants in practice.

A Mycelium System is parameterized by the following:
- `SizeBits` is the number of Bits used to represent size in the system.
- `RefSize` an Integer, the number of bits needed to encode a *Ref*.
- `MaxSize` must be >= `2 * RefSize`.  The maximum size of any Value in the System.
- `PortSize` an Integer, the number of bits needed to encode a Port.
- `XOF` a keyed [Extendable Output Function](https://en.wikipedia.org/wiki/Extendable-output_function).

All of the Mycelium Values that are created and operated on with the same parameters set are part of the same System.

### Reference implementation
The reference implementations use the following parameter set.

```
  XOF       => BLAKE3
  SizeBits  => 32
  RefSize   => 256
  MaxSize   => 2^24
  PortSize  => 256
```
