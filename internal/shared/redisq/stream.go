// Package redisq — Django web'dan keladigan on-demand tasklar uchun Redis Stream
// consumer. Kontrakt: XADD <stream> payload='{"task":"...","args":{...}}'.
// Consumer group bilan o'qiladi — worker o'lsa pending xabarlar qayta yetkaziladi.
package redisq

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Task struct {
	Task string          `json:"task"`
	Args json.RawMessage `json:"args"`
}

type Handler func(ctx context.Context, args json.RawMessage) error

type Consumer struct {
	rdb      *redis.Client
	stream   string
	group    string
	consumer string
	handlers map[string]Handler
	log      *zap.Logger
}

func NewConsumer(redisURL, stream, group string, log *zap.Logger) (*Consumer, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		rdb:      redis.NewClient(opts),
		stream:   stream,
		group:    group,
		consumer: "worker-1", // bitta replica; ko'paysa pod nomi qo'yiladi
		handlers: map[string]Handler{},
		log:      log,
	}, nil
}

func (c *Consumer) Register(task string, h Handler) { c.handlers[task] = h }

func (c *Consumer) Run(ctx context.Context) error {
	// Group yo'q bo'lsa yaratamiz (stream ham MKSTREAM bilan).
	if err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err(); err != nil &&
		!errors.Is(err, redis.Nil) && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}

	// Avval pending (o'lib qolgan run'dan qolgan) xabarlarni qayta ishlaymiz.
	c.consume(ctx, "0")

	for ctx.Err() == nil {
		c.consume(ctx, ">")
	}
	return ctx.Err()
}

func (c *Consumer) consume(ctx context.Context, id string) {
	res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, id},
		Count:    10,
		Block:    5 * time.Second,
	}).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) && ctx.Err() == nil {
			c.log.Warn("xreadgroup", zap.Error(err))
			time.Sleep(2 * time.Second)
		}
		return
	}
	for _, stream := range res {
		for _, msg := range stream.Messages {
			c.handle(ctx, msg)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg redis.XMessage) {
	payload, _ := msg.Values["payload"].(string)
	var t Task
	if err := json.Unmarshal([]byte(payload), &t); err != nil {
		c.log.Error("bad payload", zap.String("id", msg.ID), zap.Error(err))
		c.rdb.XAck(ctx, c.stream, c.group, msg.ID) // parse bo'lmaydigan xabarni qayta urinish befoyda
		return
	}
	h, ok := c.handlers[t.Task]
	if !ok {
		c.log.Error("unknown task", zap.String("task", t.Task))
		c.rdb.XAck(ctx, c.stream, c.group, msg.ID)
		return
	}
	if err := h(ctx, t.Args); err != nil {
		// Ack QILMAYMIZ — restart'dan keyin pending sifatida qayta o'qiladi.
		// ponytail: per-task retry-limit yo'q, handler'lar ichida o'zi hal qiladi.
		c.log.Error("task failed", zap.String("task", t.Task), zap.Error(err))
		return
	}
	c.rdb.XAck(ctx, c.stream, c.group, msg.ID)
}
