package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.com/moderntoken/gateways/decimal"
)

type Accounts_SelectWithPortfolioTriggersRow struct {
	ID           int64
	Name         string
	ExchangeName string
	Key          string
	Secret       string
	Passphrase   *string
	Triggers     []struct {
		ID             uuid.UUID
		Type           string
		Currency       string
		CreatedAt      time.Time
		Limit          *decimal.Decimal
		Percent        *decimal.Decimal
		StartTotalCost *decimal.Decimal
		TrailingAlert  bool
	} `json:"-"`
}

func (q *Queries) Accounts_SelectWithPortfolioTriggers(ctx context.Context) ([]Accounts_SelectWithPortfolioTriggersRow, error) {
	query := `select a.id a_id, a.name, a.exchange_name, a.key, a.secret, a.passphrase,
		pt.id pt_id, pt.type, pt.currency, pt.created_at, pt.limit::numeric, pt.percent, pt.start_total_cost, pt.trailing_alert
		from accounts a
		left join portfolio_triggers pt on pt.portfolio_id = a.id;`
	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query accounts")
	}

	defer rows.Close()

	type Result struct {
		ID           int64
		Name         string
		ExchangeName string
		Key          string
		Secret       string
		Passphrase   *string
		// Left join
		PortfolioID    *uuid.UUID
		Type           *string
		Currency       *string
		Limit          *decimal.Decimal
		Percent        *decimal.Decimal
		StartTotalCost *decimal.Decimal
		TrailingAlert  *bool
		CreatedAt      *time.Time
	}

	var accs []Accounts_SelectWithPortfolioTriggersRow
	m := make(map[int64]*Accounts_SelectWithPortfolioTriggersRow)

	for rows.Next() {
		var res Result
		if err := rows.Scan(
			&res.ID,
			&res.Name,
			&res.ExchangeName,
			&res.Key,
			&res.Secret,
			&res.Passphrase,
			&res.PortfolioID,
			&res.Type,
			&res.Currency,
			&res.CreatedAt,
			&res.Limit,
			&res.Percent,
			&res.StartTotalCost,
			&res.TrailingAlert,
		); err != nil {
			return nil, errors.Wrapf(err, "failed to scan row of %q into %T", query, res)
		}

		acc, ok := m[res.ID]
		if !ok {
			accs = append(accs, Accounts_SelectWithPortfolioTriggersRow{
				ID:           res.ID,
				Name:         res.Name,
				ExchangeName: res.ExchangeName,
				Key:          res.Key,
				Secret:       res.Secret,
				Passphrase:   res.Passphrase,
			})
			acc = &accs[len(accs)-1]
			m[res.ID] = acc
		}

		if res.PortfolioID != nil {
			acc.Triggers = append(acc.Triggers, struct {
				ID             uuid.UUID
				Type           string
				Currency       string
				CreatedAt      time.Time
				Limit          *decimal.Decimal
				Percent        *decimal.Decimal
				StartTotalCost *decimal.Decimal
				TrailingAlert  bool
			}{
				ID:             *res.PortfolioID,
				Type:           *res.Type,
				Currency:       *res.Currency,
				Limit:          res.Limit,
				Percent:        res.Percent,
				TrailingAlert:  *res.TrailingAlert,
				StartTotalCost: res.StartTotalCost,
				CreatedAt:      *res.CreatedAt,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error of query %q", query)
	}
	return accs, nil
}
