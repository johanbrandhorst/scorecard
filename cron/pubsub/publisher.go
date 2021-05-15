// Copyright 2021 Security Scorecard Authors
//
// Licensed under the Apache License, Vershandlern 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permisshandlerns and
// limitathandlerns under the License.

package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/ossf/scorecard/cron/data"
	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/gcppubsub" // Needed to link in GCP drivers.
	"google.golang.org/protobuf/encoding/protojson"
)

var errorPublish = errors.New("total errors when publishing")

type Publisher interface {
	Publish(request *data.ScorecardBatchRequest) error
	Close() error
}

func CreatePublisher(ctx context.Context, topicURL string) (Publisher, error) {
	ret := publisherImpl{}
	topic, err := pubsub.OpenTopic(ctx, topicURL)
	if err != nil {
		return &ret, fmt.Errorf("error from pubsub.OpenTopic: %w", err)
	}
	return &publisherImpl{
		ctx:   ctx,
		topic: topic,
	}, nil
}

type sender interface {
	Send(ctx context.Context, msg *pubsub.Message) error
}

type publisherImpl struct {
	ctx         context.Context
	topic       sender
	wg          sync.WaitGroup
	totalErrors uint64
}

func (publisher *publisherImpl) Publish(request *data.ScorecardBatchRequest) error {
	msg, err := protojson.Marshal(request)
	if err != nil {
		return fmt.Errorf("error from protojson.Marshal: %w", err)
	}

	publisher.wg.Add(1)
	go func() {
		defer publisher.wg.Done()
		err := publisher.topic.Send(publisher.ctx, &pubsub.Message{
			Body: msg,
		})
		if err != nil {
			log.Printf("Error when publishing message %s: %v", msg, err)
			atomic.AddUint64(&publisher.totalErrors, 1)
			return
		}
		log.Print("Successfully published message")
	}()
	return nil
}

func (publisher *publisherImpl) Close() error {
	publisher.wg.Wait()
	if publisher.totalErrors > 0 {
		return fmt.Errorf("%w: %d", errorPublish, publisher.totalErrors)
	}
	return nil
}