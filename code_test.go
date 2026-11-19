package exchange_test

import (
	"encoding"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/torhx/exchange"
)

func TestNewCode(t *testing.T) {
	t.Parallel()

	const formatErr = "must be 2 to 12 letters or digits"

	tests := []struct {
		name    string
		text    string
		want    string
		wantErr bool
	}{
		{name: "market identifier", text: "XNYS", want: "XNYS"},
		{name: "lower case", text: "xnys", want: "XNYS"},
		{name: "mixed case", text: "Xnys", want: "XNYS"},
		{name: "surrounding white space", text: " XNYS\n", want: "XNYS"},
		{name: "with a digit", text: "360T", want: "360T"},
		{name: "longer than a market identifier", text: "COINBASE", want: "COINBASE"},
		{name: "shortest", text: "XX", want: "XX"},
		{name: "longest", text: "ABCDEFGHIJKL", want: "ABCDEFGHIJKL"},
		{name: "empty", text: "", wantErr: true},
		{name: "white space alone", text: "  ", wantErr: true},
		{name: "one letter", text: "X", wantErr: true},
		{name: "thirteen letters", text: "ABCDEFGHIJKLM", wantErr: true},
		{name: "hyphen", text: "X-NYS", wantErr: true},
		{name: "space inside", text: "X NYS", wantErr: true},
		{name: "time zone", text: "America/New_York", wantErr: true},
		{name: "non-ASCII letter", text: "XNÝS", wantErr: true},
		{name: "letter that upper-cases to an ASCII one", text: "xnyſ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchange.NewCode(tt.text)
			if tt.wantErr {
				require.EqualError(t, err, `invalid exchange code "`+tt.text+`": `+formatErr)
				assert.False(t, got.IsValid())
				assert.Equal(t, exchange.Code{}, got)

				return
			}

			require.NoError(t, err)
			assert.True(t, got.IsValid())
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestCodeEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  exchange.Code
		right exchange.Code
		want  bool
	}{
		{name: "same code written two ways", left: code(t, "XNYS"), right: code(t, " xnys "), want: true},
		{name: "different codes", left: code(t, "XNYS"), right: code(t, "XNAS"), want: false},
		{name: "code against the zero value", left: code(t, "XNYS"), right: exchange.Code{}, want: false},
		{name: "code of a built-in exchange", left: code(t, "XLON"), right: builtIn(t, "XLON").Code(), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.left == tt.right)
		})
	}
}

func TestCodeText(t *testing.T) {
	t.Parallel()

	var (
		_ encoding.TextMarshaler   = exchange.Code{}
		_ encoding.TextUnmarshaler = &exchange.Code{}
	)

	tests := []struct {
		name string
		code exchange.Code
		want string
	}{
		{name: "market identifier", code: code(t, "XNYS"), want: "XNYS"},
		{name: "code of an exchange outside the built-in list", code: code(t, "KRAKEN"), want: "KRAKEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := tt.code.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(text))

			value, err := tt.code.Value()
			require.NoError(t, err)
			assert.Equal(t, tt.want, value)

			var parsed exchange.Code

			require.NoError(t, parsed.UnmarshalText(text))
			assert.Equal(t, tt.code, parsed)
		})
	}
}

func TestCodeJSON(t *testing.T) {
	t.Parallel()

	type payload struct {
		Venue   exchange.Code         `json:"venue"`
		Volumes map[exchange.Code]int `json:"volumes"`
	}

	in := payload{Venue: code(t, "XNYS"), Volumes: map[exchange.Code]int{code(t, "KRAKEN"): 7}}

	data, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"venue":"XNYS","volumes":{"KRAKEN":7}}`, string(data))

	var out payload

	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, in, out)
}

func TestCodeScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		src     any
		want    exchange.Code
		wantErr string
	}{
		{name: "string", src: "XLON", want: code(t, "XLON")},
		{name: "bytes in lower case", src: []byte("kraken"), want: code(t, "KRAKEN")},
		{name: "null", src: nil, wantErr: "invalid exchange code: NULL"},
		{name: "unsupported type", src: int64(1), wantErr: "invalid exchange code: unsupported type int64"},
		{name: "invalid string", src: "N", wantErr: `invalid exchange code "N": must be 2 to 12 letters or digits`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := code(t, "XNYS")
			got := before

			err := got.Scan(tt.src)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, before, got, "a failed scan must leave the code unchanged")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCodeNamedValues(t *testing.T) {
	t.Parallel()

	exchanges, err := exchange.Exchanges()
	require.NoError(t, err)

	builtIn := make([]exchange.Code, len(exchanges))
	for i, e := range exchanges {
		builtIn[i] = e.Code()
	}

	assert.Equal(t, builtIn, namedCodes, "every built-in exchange must have a named code, in the order of the list")
}

// namedCodes lists every named code in the order of the built-in list.
var namedCodes = []exchange.Code{
	exchange.XNYS, exchange.XNAS, exchange.ARCX, exchange.XASE, exchange.BATS, exchange.XCBO, exchange.XCBF, exchange.XTSE,
	exchange.XLON, exchange.XPAR, exchange.XAMS, exchange.XETR, exchange.XSWX, exchange.XTKS, exchange.XHKG, exchange.XSHG,
	exchange.XKRX, exchange.XBOM, exchange.XSES, exchange.XASX, exchange.BINANCE, exchange.COINBASE,
}
