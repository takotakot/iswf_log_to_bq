package untar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	common "github.com/takotakot/iswf_log_to_bq/common/go"

	"cloud.google.com/go/storage"
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"golang.org/x/sync/errgroup"
)

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("HandleUntarEvent", HandleUntarEvent)
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type PubSubMessage struct {
	ID string
	Data []byte `json:"data"`
	Attributes map[string]string
	PublishTime time.Time
	DeliveryAttempt *int
	OrderingKey string
}

type EnvConfig struct {
	ProjectID      string
	ContentTopicID string
	DestBucketName string
}

func NewEnvConfig() (*EnvConfig, error) {
	config := EnvConfig{}

	requiredEnv := map[string]*string{
		"PROJECT_ID":       &config.ProjectID,
		"CONTENT_TOPIC_ID": &config.ContentTopicID,
		"DEST_BUCKET_NAME": &config.DestBucketName,
	}

	for envVar, valuePtr := range requiredEnv {
		value := os.Getenv(envVar)
		if value == "" {
			return nil, fmt.Errorf("%s environment variable is not set", envVar)
		}
		*valuePtr = value
	}

	return &config, nil
}

func HandleUntarEvent(ctx context.Context, e event.Event) error {
	envConfig, err := NewEnvConfig()
	if err != nil {
		log.Printf("Failed to load EnvConfig: %v", err)
		return fmt.Errorf("EnvConfig: %v", err)
	}

	var msg MessagePublishedData
	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}

	var fileInfo common.PubSubMessageData
	if err := json.Unmarshal(msg.Message.Data, &fileInfo); err != nil {
		return fmt.Errorf("json.Unmarshal: %v", err)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	realClient := &common.RealStorageClient{Client: client}
	return ExtractTgzAndUpload(ctx, realClient, fileInfo.Bucket, fileInfo.FilePath, envConfig.DestBucketName, common.PubSubMessageSenderFactory(ctx, envConfig.ProjectID, envConfig.ContentTopicID))
}

func ExtractTgzAndUpload(ctx context.Context, client common.StorageClient, srcBucketName string, srcPath string, destBucketName string, messageSender func(common.PubSubMessageData) error) error {
	r, err := client.Bucket(srcBucketName).Object(srcPath).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("NewReader: %v", err)
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	destBucket := client.Bucket(destBucketName)

	var errorGroup errgroup.Group

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // ファイルの末尾に達した
		}
		if err != nil {
			return fmt.Errorf("tar.Next: %v", err)
		}

		if header.Typeflag == tar.TypeReg {
			destObjectName := path.Join(srcPath, header.Name)
			destObject := destBucket.Object(destObjectName)
			w := destObject.NewWriter(ctx)
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return fmt.Errorf("io.Copy: %v", err)
			}
			if err := w.Close(); err != nil {
				return fmt.Errorf("Close: %v", err)
			}

			msgData := common.PubSubMessageData{
				Bucket:   destBucketName,
				FilePath: destObjectName,
			}

			errorGroup.Go(func() error {
				return messageSender(msgData)
			})
		}
	}

	if err := errorGroup.Wait(); err != nil {
		log.Printf("Failed to send Pub/Sub message: %v", err)
		return fmt.Errorf("sendPubSubMessage: %v", err)
	}

	return nil
}
