## Overview

**SQL** **P**arse: parser and formatter for rewriting foreign code embedded in SQL queries, such as parameter placeholders: `$1` or `:ident`, or code encased in delimiters: `()` `[]` `{}`. Anything the parser doesn't recognize is preserved as text.

See the full documentation at https://godoc.org/github.com/mitranim/sqlp.

## Changelog

### 0.1.4

Added `NodeWhitespace`. This is emitted for any non-zero amount of whitespace. `NodeText` now contains only non-whitespace. The performance impact seems negligible.

### 0.1.3

Support incremental parsing via `Tokenizer`. Added a few utility functions related to tree traversal. Minor breaking renaming.

### 0.1.2

Added missing `(*Error).Unwrap`.

### 0.1.1

Replaced `[]rune` with `string`. When parsing, we treat the input string as UTF-8, decoding on the fly.

### 0.1.0

First tagged release.

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
