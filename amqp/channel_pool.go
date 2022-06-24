package amqp

import (
	"context"
	"time"

	"github.com/jackc/puddle/puddleg"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

const DefaultChannelsPoolSize = 10

// ChannelPool manipulates AMQP channels pool
type ChannelPool struct {
	*puddleg.Pool[*Channel]
	size int
	conn *Connection
}

func NewPool(conn *Connection, size int) *ChannelPool {
	if size < 1 {
		size = DefaultChannelsPoolSize
	}
	return &ChannelPool{
		Pool: puddleg.NewPool(
			func(ctx context.Context) (res *Channel, err error) {
				return conn.Channel()
			},
			func(res *Channel) {
				_ = res.Close()
			},
			int32(size),
		),
		size: size,
		conn: conn,
	}
}

// Start channel pool. If RabbitMQ TCP-connection is closed pool tries to restart itself.
// Old channels are still closed but they're capable of being recreated in ChannelPool.Acquire
func (p *ChannelPool) Start() error {
	if p.conn.IsClosed() {
		return errors.WithStack(amqp.ErrClosed)
	}

	go func() {
		if err := <-p.conn.closed; err != nil {
			log.Error().Stack().Err(err).Send()

			for {
				err := p.Start()
				if err == nil {
					return
				}
				log.Error().Stack().Err(err).Send()
				time.Sleep(3 * time.Second)
			}
		}
	}()

	log.Debug().Msg("RabbitMQ channels pool is ready")
	return nil
}

// Acquire wraps (puddle.Pool).Acquire with additional logic:
// If acquired channel is closed a new channel will be acquired/created
func (p *ChannelPool) Acquire(ctx context.Context) (*puddleg.Resource[*Channel], error) {
	res, err := p.Pool.Acquire(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	select {
	case <-res.Value().closed:
		res.Hijack()
		return p.Acquire(ctx)
	default:
		return res, nil
	}
}
