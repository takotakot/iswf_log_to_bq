package common

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"fmt"
)

func SendPubSubMessage(ctx context.Context, projectId, topicId string, data PubSubMessageData) error {
	client, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("json.Marshal: %v", err)
	}

	topic := client.Topic(topicId)
	result := topic.Publish(ctx, &pubsub.Message{
		Data: jsonData,
	})

	// メッセージIDを取得してログに記録
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("get publish result: %v", err)
	}
	fmt.Printf("Published message with ID: %s\n", id)

	return nil
}

func PubSubMessageSenderFactory(ctx context.Context, projectID string, topicID string) (func (msgData PubSubMessageData) error) {
	return func (msgData PubSubMessageData) error {
		if err := SendPubSubMessage(ctx, projectID, topicID, msgData); err != nil {
			return fmt.Errorf("sendPubSubMessage: %v", err)
		}
		return nil
	}
}
