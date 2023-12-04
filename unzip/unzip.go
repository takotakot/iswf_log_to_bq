package unzip

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"

	common "github.com/takotakot/iswf_log_to_bq/common/go"

	"cloud.google.com/go/storage"
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("HandleUnzipEvent", HandleUnzipEvent)
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

func HandleUnzipEvent(ctx context.Context, e event.Event) error {
	envConfig, err := NewEnvConfig()
	if err != nil {
		log.Printf("Failed to load EnvConfig: %v", err)
		return fmt.Errorf("EnvConfig: %v", err)
	}

	log.Printf("Event ID: %s", e.ID())
	log.Printf("Event Type: %s", e.Type())

	var eventData storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &eventData); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	log.Printf("Bucket: %s", eventData.GetBucket())
	log.Printf("File: %s", eventData.GetName())
	log.Printf("Metageneration: %d", eventData.GetMetageneration())
	log.Printf("Created: %s", eventData.GetTimeCreated().AsTime())
	log.Printf("Updated: %s", eventData.GetUpdated().AsTime())

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	realClient := &common.RealStorageClient{Client: client}
	return ExtractAndUpload(ctx, realClient, eventData.GetBucket(), eventData.GetName(), eventData.GetSize(), envConfig.DestBucketName, common.PubSubMessageSenderFactory(ctx, envConfig.ProjectID, envConfig.ContentTopicID))
}

func ExtractAndUpload(ctx context.Context, client common.StorageClient, srcBucketName string, srcPath string, srcSize int64, destBucketName string, messageSender func(common.PubSubMessageData) error) error {
	srcBucket := client.Bucket(srcBucketName)
	srcObject := srcBucket.Object(srcPath)

	reader, err := srcObject.NewReader(ctx)
	if err != nil {
		log.Printf("Failed to read source object: %v", err)
		return fmt.Errorf("NewReader: %v", err)
	}
	defer reader.Close()

	// ファイルの内容をバッファに読み込む
	buf, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading file to buffer: %v", err)
	}

	// バイトスライスからReaderAtを作成
	bufReader := bytes.NewReader(buf)

	zr, err := zip.NewReader(bufReader, srcSize)
	if err != nil {
		log.Printf("Failed to create zip reader: %v", err)
		return fmt.Errorf("NewReader: %v", err)
	}

	var errorGroup errgroup.Group

	destBucket := client.Bucket(destBucketName)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			log.Printf("Failed to open file from zip: %v", err)
			return fmt.Errorf("Open: %v", err)
		}
		defer rc.Close()

		decodedName, err := url.QueryUnescape(f.Name)
		if err != nil {
			log.Printf("Failed to decode filename: %v", err)
			return fmt.Errorf("QueryUnescape: %v", err)
		}

		destObjectName := path.Join(srcPath, decodedName)
		object := destBucket.Object(destObjectName)
		w := object.NewWriter(ctx)

		_, err = io.Copy(w, rc)
		if err != nil {
			log.Printf("Failed to write to destination bucket: %v", err)
			return fmt.Errorf("Copy: %v", err)
		}

		err = w.Close()
		if err != nil {
			log.Printf("Failed to close writer: %v", err)
			return fmt.Errorf("Close: %v", err)
		}

		// ファイルが正常に保存された後、Pub/Subメッセージを送信
		msgData := common.PubSubMessageData{
			Bucket:   destBucketName,
			FilePath: destObjectName,
		}

		errorGroup.Go(func() error {
			return messageSender(msgData)
		})
	}

	if err := errorGroup.Wait(); err != nil {
		log.Printf("Failed to send Pub/Sub message: %v", err)
		return fmt.Errorf("sendPubSubMessage: %v", err)
	}

	return nil
}
