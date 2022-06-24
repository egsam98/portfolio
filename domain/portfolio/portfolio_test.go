package portfolio

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/moderntoken/gateways/core"
	"gitlab.com/moderntoken/gateways/decimal"

	"github.com/egsam98/portfolio/pg"
	"github.com/egsam98/portfolio/pg/repo"
	"github.com/egsam98/portfolio/test/mocks"
)

func TestPortfolio_Info(t *testing.T) {
	ctx := context.Background()

	data := Data{
		Prices: map[core.Currency]ConvertedTo{
			"ETH": {
				USDT: decimal.ParseDecimal("100"),
			},
		},
	}
	dataJSON, _ := json.Marshal(data)

	cmd := &redis.StringCmd{}
	cmd.SetVal(string(dataJSON))
	rdbMock := mocks.NewRedisClient(t)
	rdbMock.
		On("Get", ctx, mock.Anything).
		Return(cmd).
		Once()

	accMock := mocks.NewAccount(t)

	portf := NewPortfolio(0, "", nil, rdbMock, nil, accMock, nil)
	trigger := NewCostReachedLimit(portf, BTC, core.Amount{})
	portf.addTriggers([]Trigger{trigger})

	info, err := portf.Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, &Info{
		TriggerSettings: []TriggerSettings{trigger.Settings()},
		Data:            data,
	}, info)

	t.Run("when no data in Redis", func(t *testing.T) {
		cmd := &redis.StringCmd{}
		cmd.SetErr(redis.Nil)
		rdbMock.
			On("Get", ctx, mock.Anything).
			Return(cmd).
			Twice()
		rdbMock.
			On("Set", ctx, mock.Anything, mock.Anything, time.Duration(0)).
			Return(&redis.StatusCmd{})

		accMock.
			On("Balances").
			Return(map[core.Currency]core.Balance{}, nil)

		info, err := portf.Info(ctx)
		assert.NoError(t, err)
		assert.Equal(t, []TriggerSettings{trigger.Settings()}, info.TriggerSettings)
	})
}

func TestPortfolio_AddTriggers(t *testing.T) {
	ctx := context.Background()
	qMock := mocks.NewQuerier(t)
	db := &pg.DB{Queries: qMock}

	portf := NewPortfolio(0, "", db, nil, nil, nil, nil)
	triggers := []Trigger{
		NewCostReachedLimit(portf, USDT, core.Amount{}),
		NewCostChangedByPercent(portf, BTC, core.Amount{}, false),
	}

	qMock.
		On("PortfolioTriggers_Create", ctx, mock.Anything).
		Return(int64(len(triggers)), nil).
		Run(func(args mock.Arguments) {
			params := args.Get(1).([]repo.PortfolioTriggers_CreateParams)
			assert.Len(t, params, len(triggers))
			for i, param := range params {
				assert.Equal(t, triggers[i].ID(), param.ID)
			}
		})

	settings, err := portf.AddTriggers(context.Background(), triggers)
	assert.NoError(t, err)

	sets := make([]TriggerSettings, len(triggers))
	for i, trigger := range triggers {
		sets[i] = trigger.Settings()
	}
	assert.ElementsMatch(t, settings, sets)
	for _, trigger := range triggers {
		assert.Equal(t, trigger, portf.triggers[trigger.ID().String()])
	}
}

func TestPortfolio_Close(t *testing.T) {
	ctx := context.Background()
	accMock := mocks.NewAccount(t)
	accMock.
		On("Balances").
		Return(map[core.Currency]core.Balance{}, nil)
	accMock.
		On("NotifyBalance", mock.Anything).
		Return()
	accMock.
		On("Release").
		Return().
		Maybe()

	rdbMock := mocks.NewRedisClient(t)
	getCmd := &redis.StringCmd{}
	getCmd.SetErr(redis.Nil)
	rdbMock.
		On("Get", ctx, mock.Anything).
		Return(getCmd)
	rdbMock.
		On("Set", ctx, mock.Anything, mock.Anything, time.Duration(0)).
		Return(&redis.StatusCmd{})
	rdbMock.
		On("Del", ctx, mock.Anything).
		Return(&redis.IntCmd{})

	portf := NewPortfolio(1, "", nil, rdbMock, nil, accMock, nil)
	assert.NoError(t, portf.start())
	portf.Close(false)
	assert.EqualValues(t, 1, portf.closed)

	t.Run("when destroy", func(t *testing.T) {
		qMock := mocks.NewQuerier(t)
		db := &pg.DB{Queries: qMock}
		qMock.
			On("PortfolioTriggers_DeleteByPortfolioID", context.Background(), int64(1)).
			Return(nil)

		portf := NewPortfolio(1, "", db, rdbMock, nil, accMock, nil)
		assert.NoError(t, portf.start())
		portf.Close(true)
		assert.EqualValues(t, 1, portf.closed)
		assert.Eventually(t, func() bool {
			return qMock.AssertExpectations(t)
		}, time.Second, time.Millisecond)
	})
}

func TestPortfolio_IsClosed(t *testing.T) {
	accMock := mocks.NewAccount(t)
	accMock.
		On("Balances").
		Return(map[core.Currency]core.Balance{}, nil)
	accMock.
		On("NotifyBalance", mock.Anything).
		Return()
	accMock.
		On("Release").
		Return().
		Maybe()

	rdbMock := mocks.NewRedisClient(t)
	getCmd := &redis.StringCmd{}
	getCmd.SetErr(redis.Nil)
	rdbMock.
		On("Get", context.Background(), mock.Anything).
		Return(getCmd)
	rdbMock.
		On("Set", context.Background(), mock.Anything, mock.Anything, time.Duration(0)).
		Return(&redis.StatusCmd{})

	portf := NewPortfolio(0, "", nil, rdbMock, nil, accMock, nil)
	assert.True(t, portf.IsClosed())
	assert.NoError(t, portf.start())
	assert.False(t, portf.IsClosed())
	portf.Close(false)
	assert.True(t, portf.IsClosed())
}

func TestPortfolio_price(t *testing.T) {
	ethDogeSymbol := core.Symbol{Base: "ETH", Quote: "DOGE"}
	ethDogePrice := decimal.NewDecimal(105, 1)

	ethDoge := mocks.NewInstrument(t)
	ethDoge.
		On("Price").
		Return(decimal.Decimal{}, ethDogePrice)

	gwMock := mocks.NewGateway(t)
	gwMock.
		On("Instrument", ethDogeSymbol.String()).
		Return(ethDoge, nil).
		Once()

	portf := NewPortfolio(0, "", nil, nil, gwMock, nil, nil)
	price := portf.price(ethDogeSymbol.Base, ethDogeSymbol.Quote, 0)
	assert.True(t, ethDogePrice.Eq(price))

	t.Run("when same symbols", func(t *testing.T) {
		price := portf.price(ethDogeSymbol.Base, ethDogeSymbol.Base, 0)
		assert.True(t, price.Eq(decimal.NewDecimal(1, 0)))
	})

	t.Run("when reverse", func(t *testing.T) {
		gwMock.
			On("Instrument", core.Symbol{Base: ethDogeSymbol.Quote, Quote: ethDogeSymbol.Base}.String()).
			Return(nil, errors.New("")).
			Once()
		gwMock.
			On("Instrument", ethDogeSymbol.String()).
			Return(ethDoge, nil).
			Once()

		price := portf.price(ethDogeSymbol.Quote, ethDogeSymbol.Base, 0)
		assert.True(t, decimal.NewDecimal(1, 0).Div(ethDogePrice).Eq(price))
	})

	t.Run("when recursive", func(t *testing.T) {
		gwMock.
			On("Instrument", ethDogeSymbol.String()).
			Return(nil, errors.New("")).
			Once()
		gwMock.
			On("Instrument", core.Symbol{Base: ethDogeSymbol.Quote, Quote: ethDogeSymbol.Base}.String()).
			Return(nil, errors.New("")).
			Once()
		gwMock.
			On("AllSymbols").
			Return([]core.Symbol{
				{
					Base: "ETH", Quote: "USDT",
				},
			}).
			Once()
		usdtDoge := mocks.NewInstrument(t)
		usdtDogePrice := decimal.NewDecimal(200, 0)
		usdtDoge.
			On("Price").
			Return(decimal.Decimal{}, usdtDogePrice)
		gwMock.
			On("Instrument", core.Symbol{Base: "USDT", Quote: ethDogeSymbol.Quote}.String()).
			Return(usdtDoge, nil).
			Once()
		ethUsdt := mocks.NewInstrument(t)
		ethUsdtPrice := decimal.NewDecimal(200, 0)
		ethUsdt.
			On("Price").
			Return(decimal.Decimal{}, ethUsdtPrice)
		gwMock.
			On("Instrument", core.Symbol{Base: "ETH", Quote: "USDT"}.String()).
			Return(ethUsdt, nil)

		price := portf.price(ethDogeSymbol.Base, ethDogeSymbol.Quote, 0)
		// ETHDOGE = ETHUSDT * USDTDOGE
		assert.True(t, ethUsdtPrice.Mul(usdtDogePrice).Eq(price))
	})
}
