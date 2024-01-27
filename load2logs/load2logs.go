package load2logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	common "github.com/takotakot/iswf_log_to_bq/common/go"

	"cloud.google.com/go/bigquery"
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("HandleLoadEvent", HandleLoadEvent)
	functions.CloudEvent("HandleLogLoadEvent", HandleLogLoadEvent)
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type PubSubMessage struct {
	ID              string
	Data            []byte `json:"data"`
	Attributes      map[string]string
	PublishTime     time.Time
	DeliveryAttempt *int
	OrderingKey     string
}

type EnvConfig struct {
	ProjectID string
	DatasetID string
	TableID   string
}

func NewEnvConfig() (*EnvConfig, error) {
	config := EnvConfig{}

	requiredEnv := map[string]*string{
		"PROJECT_ID": &config.ProjectID,
		"DATASET_ID": &config.DatasetID,
		"TABLE_ID":   &config.TableID,
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

func HandleLoadEvent(ctx context.Context, e event.Event) error {
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

	srcFileId := "gs://" + fileInfo.Bucket + "/" + fileInfo.FilePath

	client, err := bigquery.NewClient(ctx, envConfig.ProjectID)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return fmt.Errorf("bigquery.NewClient: %v", err)
	}
	defer client.Close()

	realClient := &common.RealBigQueryClient{Client: client}

	return Load2Bq(ctx, realClient, srcFileId, envConfig.DatasetID, envConfig.TableID)
}

func HandleLogLoadEvent(ctx context.Context, e event.Event) error {
	envConfig, err := NewEnvConfig()
	if err != nil {
		log.Printf("Failed to load EnvConfig: %v", err)
		return fmt.Errorf("EnvConfig: %v", err)
	}

	var eventData storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &eventData); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	srcFileId := "gs://" + eventData.GetBucket() + "/" + eventData.GetName()

	client, err := bigquery.NewClient(ctx, envConfig.ProjectID)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return fmt.Errorf("bigquery.NewClient: %v", err)
	}
	defer client.Close()

	realClient := &common.RealBigQueryClient{Client: client}

	return Load2Bq(ctx, realClient, srcFileId, envConfig.DatasetID, envConfig.TableID)
}

func ConstructQuery(datasetId string, tableId, uuid string) string {
	schema := `request_date STRING,
	request_time TIME,
	protocol STRING,
	client_ip STRING,
	group_name STRING,
	account_name STRING,
	reserved_1 STRING,
	transfer_status STRING,
	reserved_2 STRING,
	status_code INT64,
	fqdn STRING,
	transfer_time_ms INT64,
	request_length INT64,
	response_length INT64,
	file_type STRING,
	content_type STRING,
	categorization_reason STRING,
	determination_category STRING,
	reserved_4 STRING,
	reserved_5 STRING,
	reserved_6 STRING,
	request_url STRING,
	reserved_7 STRING,
	reserved_8 STRING,
	reserved_9 STRING`

	query := fmt.Sprintf(`BEGIN
	CREATE TABLE `+"`%s.%s`"+`
	(
	%s
	);

	LOAD DATA INTO `+"`%s.%s`"+`
	(
	%s
	)
	FROM FILES (
		format = 'CSV',
		uris = @source_uris,
		field_delimiter = '\t'
	);

	INSERT INTO `+"`%s.%s`"+`(request_time, protocol, group_name, account_name, transfer_status, status_code, fqdn, transfer_time_ms, request_length, response_length, file_type, content_type, categorization_reason, determination_category, request_url)
SELECT
	TIMESTAMP(DATETIME(PARSE_DATE('%%Y/%%m/%%d', request_date), request_time), 'Asia/Tokyo') AS request_time,
	protocol,
	group_name,
	account_name,
	transfer_status,
	status_code,
	fqdn,
	transfer_time_ms,
	request_length,
	response_length,
	CASE file_type
		WHEN '-' THEN NULL
		ELSE file_type
		END AS file_type,
	CASE content_type
		WHEN '-' THEN NULL
		ELSE content_type
		END AS content_type,
	categorization_reason,
	determination_category,
	request_url
FROM `+"`%s.%s`;"+`
DROP TABLE IF EXISTS `+"`%s.%s`;"+`
END
`, datasetId, uuid, schema, datasetId, uuid, schema, datasetId, tableId, datasetId, uuid, datasetId, uuid)

	return query
}

func Load2Bq(ctx context.Context, client common.BigQueryClient, srcFileId string, datasetId string, tableId string) error {
	query := ConstructQuery(datasetId, tableId, uuid.New().String())
	q := client.Query(query)
	q.SetParameters([]bigquery.QueryParameter{
		{
			Name:  "source_uris",
			Value: []string{srcFileId},
		},
	})

	job, err := q.Run(ctx)
	if err != nil {
		log.Printf("Failed to Run query:%v", err)
		return fmt.Errorf("Run: %v", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		log.Printf("Job failed:%v", err)
		return fmt.Errorf("Job err: %v", err)
	}
	if status.Err() != nil {
		log.Printf("Job status error:%v", status.Err())
		return fmt.Errorf("Job status error: %v", status.Err())
	}

	return nil
}
