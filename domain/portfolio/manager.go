package portfolio

import (
	"context"
	"sync"

	"github.com/egsam98/portfolio/domain/gateways"
	"github.com/egsam98/portfolio/pg"
	"github.com/egsam98/portfolio/pg/repo"
	"github.com/go-redis/redis/v9"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/moderntoken/gateways/core"
)

// Manager holds and controls multiple Portfolio-s
type Manager struct {
	db             *pg.DB
	rdb            redis.UniversalClient
	gwsMngr        gateways.Manager
	portfolios     map[string]*Portfolio // account name is a key
	portfoliosMu   sync.RWMutex
	eventPublisher TriggerEventPublisher
	logger         zerolog.Logger
}

func NewManager(db *pg.DB, rdb redis.UniversalClient, gwsMngr gateways.Manager, eventPublisher TriggerEventPublisher) *Manager {
	return &Manager{
		db:             db,
		rdb:            rdb,
		gwsMngr:        gwsMngr,
		portfolios:     make(map[string]*Portfolio),
		eventPublisher: eventPublisher,
		logger: log.Logger.With().
			Str("namespace", "portfolio_manager").
			Logger(),
	}
}

// Start loads all portfolios from database and starts them with restored triggers
func (pm *Manager) Start(ctx context.Context) error {
	accs, err := pm.db.Queries.Accounts_SelectWithPortfolioTriggers(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to select accounts with portfolio triggers")
	}

	for _, account := range accs {
		if err := pm.load(account); err != nil {
			pm.logger.Error().Stack().Err(err).Msgf("Failed to load portfolio %q", account.Name)
		}
	}
	return nil
}

// Portfolio returns portfolio registered in map by name
func (pm *Manager) Portfolio(name string) (*Portfolio, error) {
	pm.portfoliosMu.RLock()
	portf, ok := pm.portfolios[name]
	pm.portfoliosMu.RUnlock()
	if !ok {
		return nil, errors.Wrap(ErrNotFound, name)
	}
	return portf, nil
}

// AddPortfolio searches account by name in database and starts new Portfolio for it.
// Nothing happens if portfolio is registered by this name
func (pm *Manager) AddPortfolio(name string) error {
	pm.portfoliosMu.RLock()
	_, ok := pm.portfolios[name] //nolint:ifshort
	pm.portfoliosMu.RUnlock()
	if ok {
		return errors.Wrap(ErrExist, name)
	}

	account, err := pm.db.Queries.Accounts_GetByName(context.Background(), name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.Wrap(ErrAccountNotFound, name)
		}
		return errors.Wrapf(err, "failed to get account by name=%s", name)
	}

	gw, acc, err := pm.getGatewayAndAccount(account.ExchangeName, account.Name, account.Key, account.Secret,
		account.Passphrase)
	if err != nil {
		if errors.Is(err, ErrGatewayNotFound) {
			pm.logger.Warn().Err(err).Str("account", account.Name).Msg("Not supported")
			return nil
		}
		return err
	}

	portf := NewPortfolio(account.ID, account.Name, pm.db, pm.rdb, gw, acc, pm.eventPublisher)
	pm.portfoliosMu.Lock()
	if _, ok := pm.portfolios[account.Name]; !ok {
		pm.portfolios[account.Name] = portf
	}
	pm.portfoliosMu.Unlock()

	err = portf.start()
	return errors.Wrapf(err, "failed to start portfolio scheduling for account %q", account.Name)
}

// DeletePortfolio destroys portfolio (see Portfolio.Destroy) and deletes from registered map.
// Nothing happens if portfolio isn't registered by this name
func (pm *Manager) DeletePortfolio(name string) error {
	pm.portfoliosMu.RLock()
	portf, ok := pm.portfolios[name]
	pm.portfoliosMu.RUnlock()
	if !ok {
		return errors.Wrap(ErrNotFound, name)
	}

	portf.Close(true)
	pm.portfoliosMu.Lock()
	delete(pm.portfolios, name)
	pm.portfoliosMu.Unlock()
	return nil
}

// Close closes all registered portfolios
func (pm *Manager) Close() {
	pm.portfoliosMu.RLock()
	defer pm.portfoliosMu.RUnlock()
	for _, portf := range pm.portfolios {
		portf.Close(false)
	}
	// TODO: graceful
}

// load creates Portfolio from account in database and starts listening balance updates with triggers
func (pm *Manager) load(account repo.Accounts_SelectWithPortfolioTriggersRow) error {
	pm.portfoliosMu.RLock()
	_, ok := pm.portfolios[account.Name]
	pm.portfoliosMu.RUnlock()
	if ok {
		return nil
	}

	gw, acc, err := pm.getGatewayAndAccount(account.ExchangeName, account.Name, account.Key, account.Secret,
		account.Passphrase)
	if err != nil {
		if errors.Is(err, ErrGatewayNotFound) {
			pm.logger.Warn().Err(err).Str("account", account.Name).Msg("Not supported")
			return nil
		}
		return err
	}

	portf := NewPortfolio(account.ID, account.Name, pm.db, pm.rdb, gw, acc, pm.eventPublisher)

	// Set restored triggers from database
	if len(account.Triggers) > 0 {
		triggers := make([]Trigger, 0, len(account.Triggers))
		for _, dbt := range account.Triggers {
			var cur Currency
			if err := cur.Set(dbt.Currency); err != nil {
				return err
			}

			var trigger Trigger
			switch dbt.Type {
			case CCBP.String():
				if dbt.Percent == nil || dbt.StartTotalCost == nil {
					return errors.Errorf("percent and start total cost are required for trigger type %q", dbt.Type)
				}
				ccbp := new(CostChangedByPercent)
				ccbp.Restore(
					portf,
					dbt.ID,
					cur,
					*dbt.Percent,
					*dbt.StartTotalCost,
					dbt.TrailingAlert,
					dbt.CreatedAt,
				)
				trigger = ccbp
			case CRL.String():
				if dbt.Limit == nil {
					return errors.Errorf("limit is required for trigger type %q", dbt.Type)
				}
				crl := new(CostReachedLimit)
				crl.Restore(portf, dbt.ID, cur, *dbt.Limit, dbt.CreatedAt)
				trigger = crl
			default:
				continue
			}

			triggers = append(triggers, trigger)
		}

		portf.addTriggers(triggers)
	}

	pm.portfoliosMu.Lock()
	if _, ok := pm.portfolios[account.Name]; !ok {
		pm.portfolios[account.Name] = portf
	}
	pm.portfoliosMu.Unlock()

	err = portf.start()
	return errors.Wrapf(err, "failed to start portfolio scheduling for account %q", account.Name)
}

func (pm *Manager) getGatewayAndAccount(exchangeName, accName, key, secret string, passphrase *string) (core.Gateway, core.Account, error) {
	gw, ok := pm.gwsMngr.Gateway(exchangeName)
	if !ok {
		return nil, nil, errors.Wrap(ErrGatewayNotFound, exchangeName)
	}

	acc, err := gw.Account(core.Auth{
		Key:        key,
		Secret:     secret,
		Passphrase: passphrase,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidAPIKey) || errors.Is(err, core.ErrMarketClosed) {
			return nil, nil, errors.Wrapf(ErrGateway, "account %q: %s", accName, err.Error())
		}
		return nil, nil, errors.Wrapf(err, "failed to get account %q", accName)
	}

	return gw, acc, nil
}
