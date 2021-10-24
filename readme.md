## Overview

**SQL** **P**arse: parser and formatter for rewriting foreign code embedded in SQL queries, such as parameter placeholders: `$1` or `:ident`, or code encased in delimiters: `()` `[]` `{}`. Anything the parser doesn't recognize is preserved as text.

API docs: https://pkg.go.dev/github.com/mitranim/sqlp.

## Changelog

### v0.2.0

Various optimizations.

* Added `Type`, `Region`, `Token` for use by the tokenizer; see below.

* Tokenization is now allocation-free and around x2 faster in benchmarks. Instead of generating `Node` instances, the tokenizer generates stack-allocated `Token` instances.

### v0.1.4

Added `NodeWhitespace`. This is emitted for any non-zero amount of whitespace. `NodeText` now contains only non-whitespace. The performance impact seems negligible.

### v0.1.3

Support incremental parsing via `Tokenizer`. Added a few utility functions related to tree traversal. Minor breaking renaming.

### v0.1.2

Added missing `(*Error).Unwrap`.

### v0.1.1

Replaced `[]rune` with `string`. When parsing, we treat the input string as UTF-8, decoding on the fly.

### v0.1.0

First tagged release.

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
