package portfolio

import (
	"time"

	"github.com/google/uuid"
	"gitlab.com/moderntoken/gateways/decimal"
)

// CostReachedLimit is a trigger executing when portfolio's total cost reaches certain value (more than one)
type CostReachedLimit struct {
	id        uuid.UUID
	limit     decimal.Decimal
	currency  Currency
	portf     *Portfolio
	createdAt time.Time
}

func NewCostReachedLimit(portf *Portfolio, currency Currency, limit decimal.Decimal) *CostReachedLimit {
	return &CostReachedLimit{
		id:        uuid.New(),
		portf:     portf,
		currency:  currency,
		limit:     limit,
		createdAt: time.Now().UTC(),
	}
}

func (c *CostReachedLimit) ID() uuid.UUID {
	return c.id
}

// TryExecute returns non-empty ExecutionStatus if trigger is executed.
// ExecutionStatus.Done is always equal to ExecutionStatus.Ok for this type of trigger
func (c *CostReachedLimit) TryExecute() (*ExecutionStatus, error) {
	totalCost := c.portf.dataHolder.TotalBalance(c.currency)
	ok := !totalCost.LessThan(c.limit)
	return &ExecutionStatus{
		Ok:           ok,
		Done:         ok,
		CurrentValue: totalCost,
	}, nil
}

func (c *CostReachedLimit) Settings() TriggerSettings {
	return TriggerSettings{
		ID:        c.id,
		Currency:  c.currency,
		CreatedAt: c.createdAt.Unix(),
		Type:      CRL,
		Limit:     &c.limit,
	}
}

// Restore trigger state from external source (ex. database)
func (c *CostReachedLimit) Restore(
	portf *Portfolio,
	id uuid.UUID,
	currency Currency,
	limit decimal.Decimal,
	createdAt time.Time,
) {
	c.portf = portf
	c.id = id
	c.currency = currency
	c.limit = limit
	c.createdAt = createdAt
}
