package requests

import (
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/pkg/errors"
	"gitlab.com/moderntoken/gateways/decimal"
)

type AddTriggers []struct {
	Type          portfolio.TriggerType `json:"type" validate:"required" swaggertype:"string" enums:"COST_REACHED_LIMIT,COST_CHANGED_BY_PERCENT"`
	Currency      portfolio.Currency    `json:"currency" validate:"required" swaggertype:"string" enums:"USDT,BTC"`
	TrailingAlert bool                  `json:"trailing_alert"`
	Limit         *decimal.Decimal      `json:"limit"`
	Percent       *decimal.Decimal      `json:"percent"`
}

func (a AddTriggers) Validate() error {
	for _, t := range a {
		switch t.Type {
		case 0:
			return errors.New("trigger type is required")
		case portfolio.CRL:
			if t.Limit == nil || t.Limit.IsZero() {
				return errors.Errorf("limit is required for %q trigger type", t.Type)
			}
		case portfolio.CCBP:
			if t.Percent == nil || t.Percent.IsZero() {
				return errors.Errorf("percent is required for %q trigger type", t.Type)
			}
		}

		if t.Currency == 0 {
			return errors.New("currency is required")
		}
	}

	return nil
}
