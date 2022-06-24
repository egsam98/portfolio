package amqp

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

type Connection struct {
	uri, tag string
	conn     *amqp.Connection
	closed   <-chan *amqp.Error
	logger   zerolog.Logger
}

func NewConnection(amqpURI, ctag string) *Connection {
	return &Connection{
		uri:    amqpURI,
		tag:    ctag,
		logger: log.Logger.With().Str("namespace", "rabbitmq").Logger(),
	}
}

func (c *Connection) Connect() error {
	c.logger.Debug().Msgf("Dialing %q", c.uri)

	var err error
	c.conn, err = amqp.DialConfig(c.uri, amqp.Config{})
	if err != nil {
		return fmt.Errorf("DialConfig: %w", err)
	}

	c.closed = c.conn.NotifyClose(make(chan *amqp.Error, 1))

	channel, err := c.conn.Channel()
	if err != nil {
		return errors.Wrap(err, "failed to get channel")
	}

	if err := channel.Close(); err != nil {
		return err
	}

	go func() {
		if err := <-c.conn.NotifyClose(make(chan *amqp.Error, 1)); err != nil {
			c.logger.Error().Stack().Err(err).Msgf("Disconnected with error")
			for {
				err := c.Connect()
				if err == nil {
					return
				}
				c.logger.Error().Stack().Err(err).Msg("Failed to connect")
				time.Sleep(3 * time.Second)
			}
		}
	}()

	return nil
}

func (c *Connection) Channel() (*Channel, error) {
	if c.conn == nil {
		return nil, errors.WithStack(amqp.ErrClosed)
	}
	return NewChannel(c)
}

func (c *Connection) Shutdown() error {
	if c.conn == nil {
		return nil
	}

	c.logger.Info().Msg("Closing connection...")
	if err := c.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close RabbitMQ connection")
	}
	c.logger.Info().Msg("Shutdown OK")
	return nil
}

func (c *Connection) NotifyClose(receiver chan *amqp.Error) <-chan *amqp.Error {
	return c.conn.NotifyClose(receiver)
}

func (c *Connection) IsClosed() bool {
	return c.conn == nil || c.conn.IsClosed()
}
