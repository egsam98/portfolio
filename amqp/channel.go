package amqp

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/streadway/amqp"
)

type Channel struct {
	*amqp.Channel
	closed <-chan *amqp.Error
	tag    string
	logger *zerolog.Logger
}

func NewChannel(c *Connection) (*Channel, error) {
	channel, err := c.conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get channel")
	}
	return &Channel{
		Channel: channel,
		closed:  channel.NotifyClose(make(chan *amqp.Error, 1)),
		tag:     c.tag,
		logger:  &c.logger,
	}, nil
}

func (c *Channel) Publish(key string, msg interface{}) error {
	msgBody, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal %#v", msg)
	}

	envelope := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         msgBody,
	}

	keyPrefix := "marvin." + c.tag
	key = keyPrefix + "." + key
	err = c.Channel.Publish("amq.topic", key, false, false, envelope)

	if c.logger != nil {
		var e *zerolog.Event
		if err != nil {
			e = c.logger.Error()
		} else {
			e = c.logger.Debug()
		}

		e.Str("exchange", "amq.topic").
			Str("key", key).
			Bytes("body", msgBody).
			Msg("Publish")
	}

	return errors.Wrap(err, "failed to publish message")
}
