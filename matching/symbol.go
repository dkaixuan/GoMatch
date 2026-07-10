package matching

import (
	"errors"
	"strings"
)

var ErrInvalidSymbol = errors.New("invalid symbol format, expected BASE/QUOTE like ETH/USD")

// Symbol 表示一个交易币对, 如 ETH/USD。
type Symbol struct {
	Base  string // 交易的货, 如 "ETH"
	Quote string // 计价货币, 如 "USD"
}

// String 返回 "BASE/QUOTE" 格式。
func (s Symbol) String() string {
	return s.Base + "/" + s.Quote
}

// ParseSymbol 从字符串解析币对, 如 "ETH/USD" → Symbol{Base:"ETH", Quote:"USD"}。
// 格式不对返回 ErrInvalidSymbol。
//
// 你来实现。
func ParseSymbol(s string) (Symbol, error) {
	parts := strings.Split(s, "/") // 提示: 用 strings.Split
	if len(parts) != 2 {
		return Symbol{}, ErrInvalidSymbol
	}
	if parts[0] == "" || parts[1] == "" { // ← 这行必须在长度检查之后
		return Symbol{}, ErrInvalidSymbol
	}
	return Symbol{Base: parts[0], Quote: parts[1]}, nil
}
