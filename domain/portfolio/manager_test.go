package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/moderntoken/gateways/core"
	"gitlab.com/moderntoken/gateways/decimal"

	"github.com/egsam98/portfolio/pg"
	"github.com/egsam98/portfolio/pg/repo"
	"github.com/egsam98/portfolio/test/mocks"
)

func TestManager_Start(t *testing.T) {
	ctx := context.Background()
	exchangeName := "Binance.PROD"
	accName := uuid.NewString()

	qMock := mocks.NewQuerier(t)
	db := &pg.DB{Queries: qMock}
	qMock.
		On("Accounts_SelectWithPortfolioTriggers", ctx).
		Return([]repo.Accounts_SelectWithPortfolioTriggersRow{
			{
				Name:         accName,
				ExchangeName: exchangeName,
			},
		}, nil)

	pm := NewManager(db, nil, nil, nil)
	pm.portfolios[accName] = nil
	assert.NoError(t, pm.Start(ctx))
}

func TestManager_Portfolio(t *testing.T) {
	pm := NewManager(nil, nil, nil, nil)
	portfName := "test"
	portf := NewPortfolio(0, portfName, nil, nil, nil, nil, nil)
	pm.portfolios[portfName] = portf

	portfFound, err := pm.Portfolio(portfName)
	assert.NoError(t, err)
	assert.Equal(t, portf, portfFound)

	t.Run("when not found", func(t *testing.T) {
		_, err := pm.Portfolio(uuid.NewString())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestManager_AddPortfolio(t *testing.T) {
	ctx := context.Background()
	name := "test"
	qMock := mocks.NewQuerier(t)
	db := &pg.DB{Queries: qMock}
	qMock.
		On("Accounts_GetByName", ctx, name).
		Return(repo.Account{Name: name}, nil).
		Once()

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

	gwMock := mocks.NewGateway(t)
	gwMock.
		On("Account", core.Auth{}).
		Return(accMock, nil)

	gwsMngrMock := mocks.NewGatewaysManager(t)
	gwsMngrMock.
		On("Gateway", mock.Anything).
		Return(gwMock, true).
		Once()

	rdbMock := mocks.NewRedisClient(t)
	getCmd := &redis.StringCmd{}
	getCmd.SetErr(redis.Nil)
	rdbMock.
		On("Get", ctx, mock.Anything).
		Return(getCmd)
	rdbMock.
		On("Set", ctx, mock.Anything, mock.Anything, time.Duration(0)).
		Return(&redis.StatusCmd{})

	pm := NewManager(db, rdbMock, gwsMngrMock, nil)
	t.Cleanup(pm.Close)

	assert.NoError(t, pm.AddPortfolio(name))
	portf, ok := pm.portfolios[name]
	if assert.True(t, ok) {
		assert.False(t, portf.IsClosed())
	}

	t.Run("when exists", func(t *testing.T) {
		assert.ErrorIs(t, pm.AddPortfolio(name), ErrExist)
	})

	t.Run("when account doesn't exist", func(t *testing.T) {
		name := uuid.NewString()
		qMock.
			On("Accounts_GetByName", ctx, name).
			Return(repo.Account{}, pgx.ErrNoRows).
			Once()
		assert.ErrorIs(t, pm.AddPortfolio(name), ErrAccountNotFound)
	})

	t.Run("when gateway isn't supported", func(t *testing.T) {
		name := uuid.NewString()
		exchangeName := uuid.NewString()
		qMock.
			On("Accounts_GetByName", ctx, name).
			Return(repo.Account{Name: name, ExchangeName: exchangeName}, nil).
			Once()
		gwsMngrMock.
			On("Gateway", exchangeName).
			Return(nil, false).
			Once()

		assert.NoError(t, pm.AddPortfolio(name))
	})

	t.Run("when invalid account credentials", func(t *testing.T) {
		name := uuid.NewString()
		qMock.
			On("Accounts_GetByName", ctx, name).
			Return(repo.Account{Name: name}, nil).
			Once()
		gwMock := mocks.NewGateway(t)
		gwMock.
			On("Account", core.Auth{}).
			Return(nil, core.ErrInvalidAPIKey)
		gwsMngrMock.
			On("Gateway", mock.Anything).
			Return(gwMock, true).
			Once()

		assert.ErrorIs(t, pm.AddPortfolio(name), ErrGateway)
	})

	t.Run("when market is closed", func(t *testing.T) {
		name := uuid.NewString()
		qMock.
			On("Accounts_GetByName", ctx, name).
			Return(repo.Account{Name: name}, nil).
			Once()
		gwMock := mocks.NewGateway(t)
		gwMock.
			On("Account", core.Auth{}).
			Return(nil, core.ErrMarketClosed)
		gwsMngrMock.
			On("Gateway", mock.Anything).
			Return(gwMock, true).
			Once()

		assert.ErrorIs(t, pm.AddPortfolio(name), ErrGateway)
	})
}

func TestManager_DeletePortfolio(t *testing.T) {
	name := uuid.NewString()
	qMock := mocks.NewQuerier(t)
	db := &pg.DB{Queries: qMock}
	pm := NewManager(db, nil, nil, nil)
	portf := NewPortfolio(0, name, nil, nil, nil, nil, nil)
	portf.closed = 0
	pm.portfolios[name] = portf

	assert.NoError(t, pm.DeletePortfolio(name))
	assert.Empty(t, pm.portfolios)
	assert.True(t, portf.IsClosed())

	t.Run("when portfolio doesn't exist", func(t *testing.T) {
		name := uuid.NewString()
		assert.ErrorIs(t, pm.DeletePortfolio(name), ErrNotFound)
	})
}

func TestManager_Close(t *testing.T) {
	pm := NewManager(nil, nil, nil, nil)

	for i := 0; i < 2; i++ {
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

		name := uuid.NewString()
		portf := NewPortfolio(0, name, nil, rdbMock, nil, accMock, nil)
		assert.NoError(t, portf.start())
		pm.portfolios[name] = portf
	}

	pm.Close()
	for _, portf := range pm.portfolios {
		assert.True(t, portf.IsClosed())
	}
}

func TestManager_load(t *testing.T) {
	ctx := context.Background()
	name := uuid.NewString()
	exchangeName := "Binance.PROD"
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

	auth := core.Auth{
		Key:    "key",
		Secret: "secret",
	}

	gwMock := mocks.NewGateway(t)
	gwMock.
		On("Account", auth).
		Return(accMock, nil)

	gwsMngrMock := mocks.NewGatewaysManager(t)
	gwsMngrMock.
		On("Gateway", exchangeName).
		Return(gwMock, true)

	rdbMock := mocks.NewRedisClient(t)
	getCmd := &redis.StringCmd{}
	getCmd.SetErr(redis.Nil)
	rdbMock.
		On("Get", ctx, mock.Anything).
		Return(getCmd)
	rdbMock.
		On("Set", ctx, mock.Anything, mock.Anything, time.Duration(0)).
		Return(&redis.StatusCmd{})

	qMock := mocks.NewQuerier(t)
	db := &pg.DB{Queries: qMock}
	qMock.
		On("PortfolioTriggers_UpdateStartTotalCost", ctx, mock.Anything).
		Return(nil)

	mgr := NewManager(db, rdbMock, gwsMngrMock, nil)
	t.Cleanup(mgr.Close)

	limit := decimal.NewDecimal(100, 0)
	percent := decimal.NewDecimal(5, 0)
	triggerSettings := []TriggerSettings{
		{
			ID:       uuid.New(),
			Type:     CRL,
			Currency: USDT,
			Limit:    &limit,
		},
		{
			ID:             uuid.New(),
			Type:           CCBP,
			Currency:       BTC,
			Percent:        &percent,
			StartTotalCost: &limit,
			TrailingAlert:  true,
		},
	}

	expTriggerIds := make([]uuid.UUID, len(triggerSettings))
	row := repo.Accounts_SelectWithPortfolioTriggersRow{
		Name:         name,
		ExchangeName: exchangeName,
		Key:          auth.Key,
		Secret:       auth.Secret,
	}
	for i, set := range triggerSettings {
		expTriggerIds[i] = set.ID
		row.Triggers = append(row.Triggers, struct {
			ID             uuid.UUID
			Type           string
			Currency       string
			CreatedAt      time.Time
			Limit          *decimal.Decimal
			Percent        *decimal.Decimal
			StartTotalCost *decimal.Decimal
			TrailingAlert  bool
		}{
			ID:             set.ID,
			Type:           set.Type.String(),
			Currency:       set.Currency.String(),
			CreatedAt:      time.Unix(set.CreatedAt, 0),
			Limit:          set.Limit,
			Percent:        set.Percent,
			StartTotalCost: set.StartTotalCost,
			TrailingAlert:  set.TrailingAlert,
		})
	}

	err := mgr.load(row)
	assert.NoError(t, err)

	portf, ok := mgr.portfolios[name]
	if !assert.True(t, ok) {
		return
	}

	triggerIDs := make([]uuid.UUID, 0, len(portf.triggers))
	for _, tr := range portf.triggers {
		triggerIDs = append(triggerIDs, tr.ID())
	}
	assert.ElementsMatch(t, expTriggerIds, triggerIDs)
}
