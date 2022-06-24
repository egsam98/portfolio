package portfolio

import (
	"github.com/google/uuid"
	"gitlab.com/moderntoken/gateways/decimal"
)

type Trigger interface {
	ID() uuid.UUID
	TryExecute() (*ExecutionStatus, error)
	Settings() TriggerSettings
}

// ExecutionStatus
// Ok is true if trigger is executed
// Done is true if trigger is supposed to be removed
// CurrentValue is an executed value (ex. portfolio's total cost)
type ExecutionStatus struct {
	Ok           bool            `json:"ok"`
	Done         bool            `json:"done"`
	CurrentValue decimal.Decimal `json:"current_value"`
}

type TriggerSettings struct {
	ID             uuid.UUID        `json:"id" format:"UUID" validate:"required" example:"e1c6c253-00cd-4562-ae5c-ce065f8530c6"`
	Type           TriggerType      `json:"type" validate:"required" swaggertype:"string" enums:"COST_REACHED_LIMIT,COST_CHANGED_BY_PERCENT"`
	CreatedAt      int64            `json:"created_at" validate:"required" format:"timestamp"`
	Currency       Currency         `json:"currency" validate:"required" swaggertype:"string" enums:"USDT,BTC"`
	Limit          *decimal.Decimal `json:"limit,omitempty"`
	Percent        *decimal.Decimal `json:"percent,omitempty"`
	StartTotalCost *decimal.Decimal `json:"start_total_cost,omitempty"`
	TrailingAlert  bool             `json:"trailing_alert"`
}
