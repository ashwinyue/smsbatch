package queue

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/ashwinyue/dcp/internal/pkg/log"
	"github.com/segmentio/kafka-go"
)

// ConsumeHandler defines the interface for message consumption
type ConsumeHandler interface {
	Consume(val any) error
}

// WaitGroupWrapper wraps sync.WaitGroup for easier goroutine management
type WaitGroupWrapper struct {
	sync.WaitGroup
}

// Wrap executes a function in a goroutine and manages the WaitGroup
func (w *WaitGroupWrapper) Wrap(fn func()) {
	w.Add(1)
	go func() {
		defer w.Done()
		fn()
	}()
}

// KQueue represents a Kafka queue with consumer and producer capabilities
type KQueue struct {
	consumer         *kafka.Reader
	handler          ConsumeHandler
	producerRoutines WaitGroupWrapper
	consumerRoutines WaitGroupWrapper
	forceCommit      bool
	channel          chan kafka.Message
	processors       int
	consumers        int
	ctx              context.Context
	cancel           context.CancelFunc
}

// KafkaConfig holds configuration for Kafka queue
type KafkaConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	Processors    int
	Consumers     int
	ForceCommit   bool
	QueueCapacity int
	MinBytes      int
	MaxBytes      int
}

// NewKQueue creates a new Kafka queue instance
func NewKQueue(config *KafkaConfig, handler ConsumeHandler) (*KQueue, error) {
	if len(config.Brokers) == 0 {
		return nil, errors.New("kafka brokers cannot be empty")
	}
	if config.Topic == "" {
		return nil, errors.New("kafka topic cannot be empty")
	}
	if handler == nil {
		return nil, errors.New("consume handler cannot be nil")
	}

	// Set default values
	if config.Processors <= 0 {
		config.Processors = 8
	}
	if config.Consumers <= 0 {
		config.Consumers = 8
	}
	if config.QueueCapacity <= 0 {
		config.QueueCapacity = 100
	}
	if config.MinBytes <= 0 {
		config.MinBytes = 1
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 1024 * 1024 // 1MB
	}

	readerConfig := kafka.ReaderConfig{
		Brokers:       config.Brokers,
		Topic:         config.Topic,
		GroupID:       config.GroupID,
		QueueCapacity: config.QueueCapacity,
		MinBytes:      config.MinBytes,
		MaxBytes:      config.MaxBytes,
	}

	ctx, cancel := context.WithCancel(context.Background())

	queue := &KQueue{
		consumer:    kafka.NewReader(readerConfig),
		handler:     handler,
		channel:     make(chan kafka.Message, config.QueueCapacity),
		forceCommit: config.ForceCommit,
		processors:  config.Processors,
		consumers:   config.Consumers,
		ctx:         ctx,
		cancel:      cancel,
	}

	return queue, nil
}

// Start begins consuming messages from Kafka
func (q *KQueue) Start() {
	log.Infow("Starting Kafka queue")
	go q.startProducers()
	go q.startConsumers()

	q.producerRoutines.Wait()
	close(q.channel)
	q.consumerRoutines.Wait()
	log.Infow("Kafka queue stopped")
}

// Stop gracefully stops the Kafka queue
func (q *KQueue) Stop() {
	log.Infow("Stopping Kafka queue")
	q.cancel()
	if q.consumer != nil {
		q.consumer.Close()
	}
}

// startProducers starts goroutines to fetch messages from Kafka
func (q *KQueue) startProducers() {
	for i := 0; i < q.consumers; i++ {
		q.producerRoutines.Wrap(func() {
			for {
				select {
				case <-q.ctx.Done():
					return
				default:
					msg, err := q.consumer.FetchMessage(q.ctx)
					// io.EOF means consumer closed
					// io.ErrClosedPipe means committing messages on the consumer,
					// kafka will refire the messages on uncommitted messages, ignore
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
						return
					}

					if err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						log.Errorw("Error on reading message", "error", err)
						continue
					}

					select {
					case q.channel <- msg:
					case <-q.ctx.Done():
						return
					}
				}
			}
		})
	}
}

// startConsumers starts goroutines to process messages
func (q *KQueue) startConsumers() {
	for i := 0; i < q.processors; i++ {
		q.consumerRoutines.Wrap(func() {
			for {
				select {
				case msg, ok := <-q.channel:
					if !ok {
						return
					}
					q.processMessage(msg)
				case <-q.ctx.Done():
					return
				}
			}
		})
	}
}

// processMessage handles individual message processing
func (q *KQueue) processMessage(msg kafka.Message) {
	if err := q.handler.Consume(msg); err != nil {
		log.Errorw("consume message failed", "message", string(msg.Value), "error", err)
		if !q.forceCommit {
			return
		}
	}

	if err := q.consumer.CommitMessages(q.ctx, msg); err != nil {
		log.Errorw("commit message failed", "error", err)
	}
}
