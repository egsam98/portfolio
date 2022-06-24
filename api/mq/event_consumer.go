package mq

import (
	"context"
	"encoding/json"
	"time"

	amqpx "github.com/egsam98/portfolio/amqp"
	"github.com/egsam98/portfolio/domain"
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

const EventQueue = "portfolio.events"

type Event struct {
	Event       PortfolioEvent `json:"event"`
	AccountName string         `json:"account_name"`
}

// EventConsumer receives Event-s related with accounts via AMQP protocol from EventQueue
type EventConsumer struct {
	id, serverName string
	pool           *amqpx.ChannelPool
	pm             *portfolio.Manager
	logger         zerolog.Logger
}

func NewEventConsumer(serverName string, pool *amqpx.ChannelPool, pm *portfolio.Manager) *EventConsumer {
	id := EventQueue + "." + serverName
	return &EventConsumer{
		id:         id,
		serverName: serverName,
		pool:       pool,
		pm:         pm,
		logger: log.Logger.With().
			Str("namespace", "consumer").
			Str("consumer_id", id).
			Logger(),
	}
}

// Start consuming Event-s in goroutine.
// If AMQP channel is closed consumer tries to resubscribe updates every 5s
func (ec *EventConsumer) Start(ctx context.Context) {
	go func() {
		for {
			err := ec.listen(ctx)
			if err == nil {
				return
			}

			log.Error().Stack().Err(err).Send()
			time.Sleep(5 * time.Second)
		}
	}()
}

// listen messages from EventQueue
func (ec *EventConsumer) listen(ctx context.Context) error {
	res, err := ec.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer res.Release()
	channel := res.Value()

	if err := channel.Qos(5, 0, false); err != nil {
		return errors.WithStack(err)
	}

	msgs, err := channel.Consume(
		EventQueue,
		ec.id,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to consume from %q", EventQueue)
	}

	defer func() {
		if err := channel.Cancel(ec.id, false); err != nil {
			ec.logger.Err(err).Msg("Failed to cancel consumer")
		}
	}()

	ec.logger.Info().Msgf("Consuming from %q...", EventQueue)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return errors.WithStack(amqp.ErrClosed)
			}

			ec.logger.Info().
				Str("id", msg.MessageId).
				Bytes("body", msg.Body).
				Msg("Received message")

			if requeue, err := ec.handleMessage(&msg); err != nil {
				var e *zerolog.Event
				if requeue {
					e = ec.logger.Error().Stack()
				} else {
					e = ec.logger.Debug()
				}
				_ = msg.Reject(requeue)
				e.Err(err).
					Str("id", msg.MessageId).
					Bool("requeue", requeue).
					Bytes("body", msg.Body).
					Msg("Rejected message")
				continue
			}

			_ = msg.Ack(false)
		}
	}
}

func (ec *EventConsumer) handleMessage(msg *amqp.Delivery) (requeue bool, err error) {
	var event Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return false, errors.Wrapf(err, "failed to unmarshal %s into %T", string(msg.Body), ec)
	}

	switch event.Event {
	case AccountCreated:
		err = ec.pm.AddPortfolio(event.AccountName)
	case AccountDeleted:
		err = ec.pm.DeletePortfolio(event.AccountName)
	default:
		return false, errors.Errorf("unknown event: %s", event.Event)
	}

	if err != nil && !errors.As(err, new(domain.Error)) {
		requeue = true
	}
	return
}
