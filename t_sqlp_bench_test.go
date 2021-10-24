package sqlp

import (
	"bytes"
	"testing"
)

func Benchmark_tokenizeHugeQuery(b *testing.B) {
	for range counter(b.N) {
		benchTokenizeHugeQuery()
	}
}

//go:noinline
func benchTokenizeHugeQuery() {
	tokenizer := Tokenizer{Source: hugeQuery}
	for {
		tok := tokenizer.Token()
		if tok.IsInvalid() {
			break
		}
	}
}

func Benchmark_parseHugeQuery(b *testing.B) {
	for range counter(b.N) {
		_ = benchParseHugeQuery()
	}
}

//go:noinline
func benchParseHugeQuery() Nodes {
	val, err := Parse(hugeQuery)
	try(err)
	return val
}

func Benchmark_remakeHugeQuery(b *testing.B) {
	for range counter(b.N) {
		benchRemakeHugeQuery()
	}
}

//go:noinline
func benchRemakeHugeQuery() {
	var buf bytes.Buffer
	for _, char := range hugeQuery {
		buf.WriteRune(char)
	}
}

func Benchmark_formatHugeQuery(b *testing.B) {
	for range counter(b.N) {
		benchFormatHugeQuery()
	}
}

var hugeQueryNodes = benchParseHugeQuery()

//go:noinline
func benchFormatHugeQuery() {
	_ = hugeQueryNodes.String()
}

const hugeQuery = /*pgsql*/ `
	select col_name
	from
		table_name

		left join table_name using (col_name)

		inner join (
			select agg(col_name) as col_name
			from table_name
			where (
				false
				or col_name = 'enum_value'
				or (:arg_one and (:arg_two or col_name = :arg_three))
			)
			group by col_name
		) as table_name using (col_name)

		left join (
			select
				table_name.col_name
			from
				table_name
				left join table_name on table_name.col_name = table_name.col_name
			where
				false
				or :arg_four::type_name is null
				or table_name.col_name between :arg_four and (:arg_four + 'literal input'::some_type)
		) as table_name using (col_name)

		left join (
			select distinct col_name as col_name
			from table_name
			where (:arg_five::type_name[] is null or col_name = any(:arg_five))
		) as table_name using (col_name)

		left join (
			select distinct col_name as col_name
			from table_name
			where (:arg_six::type_name[] is null or col_name = any(:arg_six))
		) as table_name using (col_name)
	where
		true
		and (:arg_seven or col_name in (table table_name))
		and (:arg_four :: type_name   is null or table_name.col_name is not null)
		and (:arg_five :: type_name[] is null or table_name.col_name is not null)
		and (:arg_six  :: type_name[] is null or table_name.col_name is not null)
		and (
			false
			or not col_name
			or (:arg_eight and (:arg_two or col_name = :arg_three))
		)
		and (
			false
			or not col_name
			or (:arg_nine and (:arg_two or col_name = :arg_three))
		)
		and (:arg_ten or not col_name)
		and (:arg_eleven   :: type_name is null or col_name            @@ func_name(:arg_eleven))
		and (:arg_fifteen  :: type_name is null or col_name            <> :arg_fifteen)
		and (:arg_sixteen  :: type_name is null or col_name            =  :arg_sixteen)
		and (:arg_twelve   :: type_name is null or col_name            =  :arg_twelve)
		and (:arg_thirteen :: type_name is null or func_name(col_name) <= :arg_thirteen)
	:arg_fourteen
`

func counter(val int) []struct{} { return make([]struct{}, val) }
