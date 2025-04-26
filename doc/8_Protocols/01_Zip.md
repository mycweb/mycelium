# Mycelium Zip
Mycelium Zip Files (MycZip) are a way to store Mycelium Values in [zip files](https://en.wikipedia.org/wiki/ZIP_(file_format)).
1 Value per zip file.

A MycZip file contains a single Mycelium Value with referenced Values split up across the files of the Zip File.

To be a valid MycZip file all of the following must apply:
- Contain a file at path `root`.
- The contents of the file `root` must be an `AnyValue` without any extra leading or trailing bits.
  This is known as the *root*.
- All of the Values transitively referenced by the root value, must
  have their own file with path = `base64(post(value))`

A consequence of the *root* being an *AnyValue* is that there must be at least 2 other files in the file.

The base64 encoding for paths uses an order preserving alphabet, which is atypical for base64.

Mycelium Zip files containing the same Value will not necessarily contain identical bytes.
Various features of zip files can add extra information--none of which is part of the format.
