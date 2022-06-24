package mq

import (
	"github.com/pkg/errors"
)

type PortfolioEvent uint8

const (
	AccountCreated PortfolioEvent = iota + 1
	AccountDeleted
)

var (
	portfolioEventKeyValues = map[PortfolioEvent]string{
		AccountCreated: "ACCOUNT_CREATED",
		AccountDeleted: "ACCOUNT_DELETED",
	}
	portfolioEventValueKeys = map[string]PortfolioEvent{
		"ACCOUNT_CREATED": AccountCreated,
		"ACCOUNT_DELETED": AccountDeleted,
	}
)

func (p PortfolioEvent) String() string {
	return portfolioEventKeyValues[p]
}

func (p PortfolioEvent) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *PortfolioEvent) UnmarshalText(text []byte) error {
	txt := string(text)
	if currency, ok := portfolioEventValueKeys[txt]; ok {
		*p = currency
		return nil
	}
	return errors.Errorf("invalid portfolio event: %s", txt)
}
