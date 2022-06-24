package portfolio

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/moderntoken/gateways/core"
	"gitlab.com/moderntoken/gateways/decimal"

	"github.com/egsam98/portfolio/pg"
	"github.com/egsam98/portfolio/pg/repo"
)

// Portfolio holds account's balances converted to Currency types + prices converted as well.
// Portfolio supports Trigger-s registration that can be executed on account's balance update and
// reported with TriggerEventPublisher
type Portfolio struct {
	closed      uint32
	id          int64
	name        string
	db          *pg.DB
	dataHolder  *dataHolder
	gw          core.Gateway
	acc         core.Account
	triggers    map[string]Trigger
	triggersMu  sync.RWMutex
	tePublisher TriggerEventPublisher
	closedCh    chan bool // true if portfolio is supposed to be destroyed
	logger      zerolog.Logger
}

type (
	TriggerEventPublisher func(event TriggerEvent) error
	Data                  struct {
		Prices  map[core.Currency]ConvertedTo `json:"prices" validate:"required"`
		Balance struct {
			Total   ConvertedTo                   `json:"total" validate:"required"`
			Details map[core.Currency]ConvertedTo `json:"details" validate:"required"`
		} `json:"balance"`
	}
	ConvertedTo  map[Currency]decimal.Decimal
	TriggerEvent struct {
		Portfolio       string          `json:"portfolio" required:"true"`
		Timestamp       int64           `json:"timestamp" required:"true" format:"timestamp"`
		CurrentValue    decimal.Decimal `json:"current_value" required:"true"`
		TriggerSettings TriggerSettings `json:"trigger_settings" required:"true"`
	}
	Info struct {
		TriggerSettings []TriggerSettings `json:"trigger_settings" validate:"required"`
		Data            Data              `json:"data" validate:"required"`
	}
)

func (d Data) MarshalBinary() ([]byte, error) {
	return json.Marshal(d)
}

func (d *Data) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, d)
}

func NewPortfolio(
	id int64,
	name string,
	db *pg.DB,
	rdb redis.UniversalClient,
	gw core.Gateway,
	acc core.Account,
	eventPublisher TriggerEventPublisher,
) *Portfolio {
	return &Portfolio{
		id:          id,
		name:        name,
		db:          db,
		dataHolder:  newDataHolder(name, rdb),
		gw:          gw,
		acc:         acc,
		tePublisher: eventPublisher,
		triggers:    make(map[string]Trigger),
		closed:      1,
		closedCh:    make(chan bool, 1),
		logger: log.Logger.With().
			Str("namespace", "portfolio").
			Int64("id", id).
			Str("name", name).
			Logger(),
	}
}

// Info returns Data + TriggerSettings
func (p *Portfolio) Info(ctx context.Context) (*Info, error) {
	settings := make([]TriggerSettings, 0, len(p.triggers))
	p.triggersMu.RLock()
	for _, trigger := range p.triggers {
		settings = append(settings, trigger.Settings())
	}
	p.triggersMu.RUnlock()

	data, err := p.dataHolder.Get(ctx)
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil, err
		}

		bals, err := p.acc.Balances()
		if err != nil {
			return nil, err
		}
		if data, err = p.updateData(bals); err != nil {
			return nil, err
		}
	}

	return &Info{
		Data:            *data,
		TriggerSettings: settings,
	}, nil
}

// AddTriggers attaches new triggers to portfolio and saves them into database.
// Triggers with same ID results an error
func (p *Portfolio) AddTriggers(ctx context.Context, triggers []Trigger) ([]TriggerSettings, error) {
	dbArgs := make([]repo.PortfolioTriggers_CreateParams, len(triggers))
	settings := make([]TriggerSettings, len(triggers))
	for i, t := range triggers {
		sets := t.Settings()
		settings[i] = sets
		dbArgs[i] = repo.PortfolioTriggers_CreateParams{
			ID:             sets.ID,
			PortfolioID:    p.id,
			Type:           sets.Type.String(),
			Currency:       sets.Currency.String(),
			CreatedAt:      time.Unix(sets.CreatedAt, 0),
			Limit:          sets.Limit,
			Percent:        sets.Percent,
			TrailingAlert:  sets.TrailingAlert,
			StartTotalCost: sets.StartTotalCost,
		}
	}
	if _, err := p.db.Queries.PortfolioTriggers_Create(ctx, dbArgs); err != nil {
		return nil, err
	}

	p.addTriggers(triggers)
	return settings, nil
}

func (p *Portfolio) Close(destroy bool) {
	if atomic.SwapUint32(&p.closed, 1) == 0 {
		p.closedCh <- destroy
	}
}

func (p *Portfolio) IsClosed() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

// addTriggers attaches triggers to internal Portfolio's triggers map
func (p *Portfolio) addTriggers(triggers []Trigger) {
	settings := make([]TriggerSettings, len(triggers))

	p.triggersMu.Lock()
	for i, t := range triggers {
		p.triggers[t.ID().String()] = t
		settings[i] = t.Settings()
	}
	p.triggersMu.Unlock()

	p.logger.Info().Interface("triggers", settings).Msg("Triggers have been registered")
}

// start listens balance updates in separate goroutine
func (p *Portfolio) start() error {
	if atomic.SwapUint32(&p.closed, 0) == 0 {
		return nil
	}

	bals, err := p.acc.Balances()
	if err != nil {
		return err
	}
	if err := p.handleBalanceUpdate(bals); err != nil {
		return err
	}

	ch := make(chan map[core.Currency]core.Balance)
	p.acc.NotifyBalance(ch)

	p.logger.Info().Msg("Portfolio has started")

	go func() {
		defer p.logger.Info().Msg("Portfolio has been closed/destroyed")
		defer p.acc.Release()

		for {
			select {
			case destroyed := <-p.closedCh:
				if destroyed {
					p.logger.Info().Msg("Destroying portfolio...")
					if err := p.db.Queries.PortfolioTriggers_DeleteByPortfolioID(context.Background(), p.id); err != nil {
						p.logger.Err(err).Msgf("Failed to delete portfolio triggers by portfolio ID=%d", p.id)
					}
					if err := p.dataHolder.Delete(context.Background()); err != nil {
						p.logger.Err(err).Msg("Failed to delete portfolio data in redis")
					}
				} else {
					p.logger.Info().Msg("Closing portfolio...")
				}
				return
			case bals, ok := <-ch:
				if !ok {
					p.logger.Info().Msgf("Closing portfolio due to gateway %s stop...", p.gw.Name())
					return
				}
				if err := p.handleBalanceUpdate(bals); err != nil {
					p.logger.Error().Stack().Err(err).Msg("Failed to handle balance update")
				}
			}
		}
	}()

	return nil
}

// handleBalanceUpdate:
// 1. It converts all currencies prices and balances to Currency types
// 2. It checks triggers state and fires TriggerEvent on execution.
// Triggers claiming to be deleted are deleted from database also
func (p *Portfolio) handleBalanceUpdate(balances map[core.Currency]core.Balance) error {
	if _, err := p.updateData(balances); err != nil {
		return err
	}

	// Check triggers
	for tID, t := range p.triggers {
		execStatus, err := t.TryExecute()
		if err != nil {
			p.logger.Error().Stack().Err(err).Msgf("Failed to execute trigger %s", t.ID())
			continue
		}

		// Trigger is executed
		if execStatus.Ok {
			p.logger.Info().
				Interface("trigger", t.Settings()).
				Interface("status", execStatus).
				Msg("Trigger has been executed")
			if p.tePublisher != nil {
				if err := p.tePublisher(TriggerEvent{
					Portfolio:       p.name,
					TriggerSettings: t.Settings(),
					Timestamp:       time.Now().Unix(),
					CurrentValue:    execStatus.CurrentValue,
				}); err != nil {
					p.logger.Error().Stack().Err(err).Msg("Failed to publish event")
				}
			}
		}

		// Trigger is done (claims to be deleted)
		if execStatus.Done {
			if err := p.db.Queries.PortfolioTriggers_Delete(context.Background(), t.ID()); err != nil {
				p.logger.Err(err).Msgf("Failed to delete portfolio trigger %q", tID)
				continue
			}
			delete(p.triggers, tID)
		}
	}

	return nil
}

// updateData updates prices and balances converted to different kinds of Currency saving them into Redis
func (p *Portfolio) updateData(balances map[core.Currency]core.Balance) (*Data, error) {
	data, err := p.dataHolder.Get(context.Background())
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil, err
		}

		data = &Data{
			Prices: map[core.Currency]ConvertedTo{},
			Balance: struct {
				Total   ConvertedTo                   `json:"total" validate:"required"`
				Details map[core.Currency]ConvertedTo `json:"details" validate:"required"`
			}{
				Total:   map[Currency]decimal.Decimal{},
				Details: map[core.Currency]ConvertedTo{},
			},
		}
	}

	for cur, bal := range balances {
		priceUSDT := p.price(cur, core.Currency(USDT.String()), 0)
		priceBTC := p.price(cur, core.Currency(BTC.String()), 0) // TODO: XBT Kraken?
		data.Prices[cur] = ConvertedTo{
			USDT: priceUSDT,
			BTC:  priceBTC,
		}
		costUSDT := bal.Available.Mul(priceUSDT)
		costBTC := bal.Available.Mul(priceBTC)
		data.Balance.Details[cur] = ConvertedTo{
			USDT: costUSDT,
			BTC:  costBTC,
		}
		data.Balance.Total[USDT] = data.Balance.Total[USDT].Add(costUSDT)
		data.Balance.Total[BTC] = data.Balance.Total[BTC].Add(costBTC)
	}

	if err := p.dataHolder.Save(context.Background(), *data); err != nil {
		return nil, err
	}
	return data, nil
}

// price calculates recursively the price of base currency in quote currency.
// depth is provided as a limit equal to 5 of recursive calls
func (p *Portfolio) price(base, quote core.Currency, depth int) decimal.Decimal {
	if base == quote {
		return decimal.NewDecimal(1, 0)
	}
	if inst, err := p.gw.Instrument(base.String() + quote.String()); err == nil {
		_, ask := inst.Price()
		return ask
	}
	if inst, err := p.gw.Instrument(quote.String() + base.String()); err == nil {
		_, ask := inst.Price()
		if ask.IsZero() {
			return decimal.Decimal{}
		}
		return decimal.NewDecimal(1, 0).Div(ask)
	}

	if depth > 5 {
		return decimal.Decimal{}
	}

	for _, symbol := range p.gw.AllSymbols() {
		if symbol.Base != base {
			continue
		}
		if price := p.price(symbol.Quote, quote, depth+1); !price.IsZero() {
			return p.price(symbol.Base, symbol.Quote, 0).Mul(price)
		}
	}

	return decimal.Decimal{}
}
