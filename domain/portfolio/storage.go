package portfolio

import (
	"context"

	"github.com/go-redis/redis/v9"
	"github.com/pkg/errors"
	"gitlab.com/moderntoken/gateways/core"
)

// dataHolder provides CRUD methods for Data stored via Redis.
// It also holds fast-accessible data parts as totalBalance
type dataHolder struct {
	rdb           redis.UniversalClient
	portfolioName string
	totalBalance  ConvertedTo
}

func newDataHolder(portfolioName string, rdb redis.UniversalClient) *dataHolder {
	return &dataHolder{
		portfolioName: portfolioName,
		totalBalance:  make(ConvertedTo),
		rdb:           rdb,
	}
}

func (s *dataHolder) Get(ctx context.Context) (*Data, error) {
	var data Data
	if err := s.rdb.Get(ctx, s.redisKey()).Scan(&data); err != nil {
		return nil, errors.Wrapf(err, "failed to get data for %s", s.portfolioName)
	}
	return &data, nil
}

func (s *dataHolder) Save(ctx context.Context, data Data) error {
	s.totalBalance = data.Balance.Total
	err := s.rdb.Set(ctx, s.redisKey(), data, 0).Err()
	return errors.Wrapf(err, "failed to save data for %s", s.portfolioName)
}

func (s *dataHolder) Delete(ctx context.Context) error {
	err := s.rdb.Del(ctx, s.redisKey()).Err()
	return errors.Wrapf(err, "failed to save data for %s", s.portfolioName)
}

func (s *dataHolder) TotalBalance(currency Currency) core.Amount {
	return s.totalBalance[currency]
}

func (s *dataHolder) redisKey() string {
	return "portfolio:" + s.portfolioName
}
