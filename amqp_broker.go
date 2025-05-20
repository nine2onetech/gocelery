// Copyright (c) 2019 Sick Yoon
// This file is part of gocelery which is released under MIT license.
// See file LICENSE for full license details.

package gocelery

import (
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPExchange stores AMQP Exchange configuration
type AMQPExchange struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
}

// NewAMQPExchange creates new AMQPExchange
func NewAMQPExchange(name string) *AMQPExchange {
	return &AMQPExchange{
		Name:       name,
		Type:       "direct",
		Durable:    true,
		AutoDelete: true,
	}
}

// AMQPQueue stores AMQP DefaultQueue configuration
type AMQPQueue struct {
	Name       string
	Durable    bool
	AutoDelete bool
}

// NewAMQPQueue creates new AMQPQueue
func NewAMQPQueue(name string) *AMQPQueue {
	return &AMQPQueue{
		Name:       name,
		Durable:    true,
		AutoDelete: false,
	}
}

// AMQPCeleryBroker is RedisBroker for AMQP
type AMQPCeleryBroker struct {
	*amqp.Channel
	Connection       *amqp.Connection
	Exchange         *AMQPExchange
	DefaultQueue     *AMQPQueue
	consumingChannel <-chan amqp.Delivery
	Rate             int
}

// NewAMQPConnection creates new AMQP channel
func NewAMQPConnection(host string) (*amqp.Connection, *amqp.Channel) {
	connection, err := amqp.Dial(host)
	if err != nil {
		panic(err)
	}

	channel, err := connection.Channel()
	if err != nil {
		panic(err)
	}
	return connection, channel
}

// NewAMQPCeleryBroker creates new AMQPCeleryBroker
func NewAMQPCeleryBroker(host string) *AMQPCeleryBroker {
	return NewAMQPCeleryBrokerByConnAndChannel(NewAMQPConnection(host))
}

// NewAMQPCeleryBrokerByConnAndChannel creates new AMQPCeleryBroker using AMQP conn and channel
func NewAMQPCeleryBrokerByConnAndChannel(conn *amqp.Connection, channel *amqp.Channel) *AMQPCeleryBroker {
	broker := &AMQPCeleryBroker{
		Channel:      channel,
		Connection:   conn,
		Exchange:     NewAMQPExchange("default"),
		DefaultQueue: NewAMQPQueue("celery"),
		Rate:         4,
	}
	if err := broker.CreateExchange(); err != nil {
		panic(err)
	}
	if err := broker.CreateQueue(); err != nil {
		panic(err)
	}
	if err := broker.Qos(broker.Rate, 0, false); err != nil {
		panic(err)
	}
	if err := broker.StartConsumingChannel(); err != nil {
		panic(err)
	}
	return broker
}

// StartConsumingChannel spawns receiving channel on AMQP queue
func (b *AMQPCeleryBroker) StartConsumingChannel() error {
	channel, err := b.Consume(b.DefaultQueue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	b.consumingChannel = channel
	return nil
}

// SendCeleryMessageToQueue sends CeleryMessage to broker
func (b *AMQPCeleryBroker) SendCeleryMessageToQueue(queueName string, message *CeleryMessage) error {
	taskMessage := message.GetTaskMessage()
	_, err := b.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // autoDelete
		false,     // exclusive
		false,     // noWait
		nil,       // args
	)
	if err != nil {
		return err
	}
	err = b.ExchangeDeclare(
		b.Exchange.Name,
		b.Exchange.Type,
		b.Exchange.Durable,
		b.Exchange.AutoDelete,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	resBytes, err := json.Marshal(taskMessage)
	if err != nil {
		return err
	}

	publishMessage := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         resBytes,
	}

	return b.Publish(
		"",
		queueName,
		false,
		false,
		publishMessage,
	)
}

// SendCeleryMessage sends CeleryMessage to broker
func (b *AMQPCeleryBroker) SendCeleryMessage(message *CeleryMessage) error {
	taskMessage := message.GetTaskMessage()
	queueName := b.DefaultQueue.Name
	_, err := b.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // autoDelete
		false,     // exclusive
		false,     // noWait
		nil,       // args
	)
	if err != nil {
		return err
	}
	err = b.ExchangeDeclare(
		b.Exchange.Name,
		b.Exchange.Type,
		b.Exchange.Durable,
		b.Exchange.AutoDelete,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	resBytes, err := json.Marshal(taskMessage)
	if err != nil {
		return err
	}

	publishMessage := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         resBytes,
	}

	return b.Publish(
		"",
		queueName,
		false,
		false,
		publishMessage,
	)
}

// GetTaskMessage retrieves task message from AMQP queue
func (b *AMQPCeleryBroker) GetTaskMessage() (*TaskMessage, error) {
	select {
	case delivery := <-b.consumingChannel:
		deliveryAck(delivery)
		var taskMessage TaskMessage
		if err := json.Unmarshal(delivery.Body, &taskMessage); err != nil {
			return nil, err
		}
		return &taskMessage, nil
	default:
		return nil, fmt.Errorf("consuming channel is empty")
	}
}

// CreateExchange declares AMQP exchange with stored configuration
func (b *AMQPCeleryBroker) CreateExchange() error {
	return b.ExchangeDeclare(
		b.Exchange.Name,
		b.Exchange.Type,
		b.Exchange.Durable,
		b.Exchange.AutoDelete,
		false,
		false,
		nil,
	)
}

// CreateQueue declares AMQP DefaultQueue with stored configuration
func (b *AMQPCeleryBroker) CreateQueue() error {
	_, err := b.QueueDeclare(
		b.DefaultQueue.Name,
		b.DefaultQueue.Durable,
		b.DefaultQueue.AutoDelete,
		false,
		false,
		nil,
	)
	return err
}
