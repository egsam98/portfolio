package portfolio

import (
	"context"
	"time"

	"github.com/egsam98/portfolio/pg/repo"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.com/moderntoken/gateways/decimal"
)

// CostChangedByPercent is a trigger executing when portfolio's total cost is changed by certain % of start value.
// If trailingAlert is true trigger becomes non-removable: executed total cost value becomes a start value for next iteration
type CostChangedByPercent struct {
	trailingAlert  bool
	portf          *Portfolio
	id             uuid.UUID
	currency       Currency
	startTotalCost decimal.Decimal
	percent        decimal.Decimal
	createdAt      time.Time
}

func NewCostChangedByPercent(portf *Portfolio, currency Currency, percent decimal.Decimal, trailingAlert bool) *CostChangedByPercent {
	cost := portf.dataHolder.TotalBalance(currency)
	return &CostChangedByPercent{
		id:             uuid.New(),
		trailingAlert:  trailingAlert,
		portf:          portf,
		currency:       currency,
		percent:        percent,
		startTotalCost: cost,
		createdAt:      time.Now().UTC(),
	}
}

func (c *CostChangedByPercent) ID() uuid.UUID {
	return c.id
}

// TryExecute returns non-empty ExecutionStatus if trigger is executed
func (c *CostChangedByPercent) TryExecute() (*ExecutionStatus, error) {
	totalCost := c.portf.dataHolder.TotalBalance(c.currency)
	devPercent := totalCost.Sub(c.startTotalCost).Abs().Div(c.startTotalCost).MulFloat(100)
	ok := !devPercent.LessThan(c.percent)

	if ok && c.trailingAlert {
		if err := c.portf.db.Queries.PortfolioTriggers_UpdateStartTotalCost(
			context.Background(),
			repo.PortfolioTriggers_UpdateStartTotalCostParams{
				StartTotalCost: &c.startTotalCost,
				ID:             c.id,
			},
		); err != nil {
			return nil, errors.Wrapf(err, "failed to update start total cost of portfolio trigger %q", c.id)
		}

		c.startTotalCost = totalCost
	}

	return &ExecutionStatus{
		Ok:           ok,
		Done:         ok && !c.trailingAlert,
		CurrentValue: devPercent,
	}, nil
}

func (c *CostChangedByPercent) Settings() TriggerSettings {
	return TriggerSettings{
		ID:             c.id,
		Currency:       c.currency,
		CreatedAt:      c.createdAt.Unix(),
		TrailingAlert:  c.trailingAlert,
		Type:           CCBP,
		Percent:        &c.percent,
		StartTotalCost: &c.startTotalCost,
	}
}

// Restore trigger state from external source (ex. database)
func (c *CostChangedByPercent) Restore(
	portf *Portfolio,
	id uuid.UUID,
	currency Currency,
	percent decimal.Decimal,
	startTotalCost decimal.Decimal,
	trailingAlert bool,
	createdAt time.Time,
) {
	c.portf = portf
	c.id = id
	c.currency = currency
	c.percent = percent
	c.trailingAlert = trailingAlert
	c.createdAt = createdAt
	c.startTotalCost = startTotalCost
}
