package matching

import (
	"errors"
	"testing"
)

func TestParseSymbol(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    Symbol
		wantErr error
	}{
		{"正常", "ETH/USD", Symbol{Base: "ETH", Quote: "USD"}, nil},
		{"BTC对", "BTC/USD", Symbol{Base: "BTC", Quote: "USD"}, nil},
		{"ETH对BTC", "ETH/BTC", Symbol{Base: "ETH", Quote: "BTC"}, nil},
		{"空字符串", "", Symbol{}, ErrInvalidSymbol},
		{"没有斜杠", "ETHUSD", Symbol{}, ErrInvalidSymbol},
		{"多个斜杠", "ETH/USD/X", Symbol{}, ErrInvalidSymbol},
		{"空base", "/USD", Symbol{}, ErrInvalidSymbol},
		{"空quote", "ETH/", Symbol{}, ErrInvalidSymbol},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseSymbol(c.input)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("ParseSymbol(%q) err = %v, 期望 %v", c.input, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSymbol(%q) 意外报错: %v", c.input, err)
			}
			if got != c.want {
				t.Errorf("ParseSymbol(%q) = %+v, 期望 %+v", c.input, got, c.want)
			}
		})
	}
}

func TestSymbolString(t *testing.T) {
	s := Symbol{Base: "ETH", Quote: "USD"}
	if got := s.String(); got != "ETH/USD" {
		t.Errorf("String() = %q, 期望 %q", got, "ETH/USD")
	}
}
