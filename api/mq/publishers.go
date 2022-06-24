package mq

import (
	"context"

	amqpx "github.com/egsam98/portfolio/amqp"
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/pkg/errors"
)

const TriggerEventKey = "portfolio.trigger_events"

// TriggerEventPublisher sends portfolio.TriggerEvent to TriggerEventKey routing key
func TriggerEventPublisher(pool *amqpx.ChannelPool) portfolio.TriggerEventPublisher {
	return func(event portfolio.TriggerEvent) error {
		res, err := pool.Acquire(context.Background())
		if err != nil {
			return err
		}
		defer res.Release()

		err = res.Value().Publish(TriggerEventKey, event)
		return errors.Wrap(err, "failed to publish")
	}
}
