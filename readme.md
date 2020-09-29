## Overview

**SQL** **P**arse: simple parser for rewriting fragments of foreign code embedded in SQL queries, such as parameter placeholders: `$1` or `:ident`, or code encased in delimiters: `()` `[]` `{}`. Anything the parser doesn't recognize is preserved as text.

See the full documentation at https://godoc.org/github.com/mitranim/sqlp.

## Changelog

### 0.1.1

Replaced `[]rune` with `string`. When parsing, we treat the input string as UTF-8, decoding on the fly.

### 0.1.0

First tagged release.

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
