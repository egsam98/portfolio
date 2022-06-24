package repo

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	Accounts_GetByName(ctx context.Context, name string) (Account, error)
	Accounts_SelectWithPortfolioTriggers(ctx context.Context) ([]Accounts_SelectWithPortfolioTriggersRow, error)
	PortfolioTriggers_Create(ctx context.Context, arg []PortfolioTriggers_CreateParams) (int64, error)
	PortfolioTriggers_Delete(ctx context.Context, id uuid.UUID) error
	PortfolioTriggers_DeleteByPortfolioID(ctx context.Context, portfolioID int64) error
	PortfolioTriggers_UpdateStartTotalCost(ctx context.Context, arg PortfolioTriggers_UpdateStartTotalCostParams) error
}
