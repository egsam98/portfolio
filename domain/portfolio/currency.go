package portfolio

import (
	"github.com/pkg/errors"
)

type Currency uint8

const (
	USDT Currency = iota + 1
	BTC
)

var (
	currencyKeyValues = map[Currency]string{
		USDT: "USDT",
		BTC:  "BTC",
	}
	currencyValueKeys = map[string]Currency{
		"USDT": USDT,
		"BTC":  BTC,
	}
)

func (c Currency) String() string {
	return currencyKeyValues[c]
}

func (c *Currency) Set(txt string) error {
	if c == nil {
		return errors.New("currency type is nil")
	}
	if val, ok := currencyValueKeys[txt]; ok {
		*c = val
		return nil
	}
	return errors.Errorf("invalid currency: %s", txt)
}

func (c Currency) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c *Currency) UnmarshalText(text []byte) error {
	txt := string(text)
	if currency, ok := currencyValueKeys[txt]; ok {
		*c = currency
		return nil
	}
	return errors.Errorf("invalid currency: %s", txt)
}
