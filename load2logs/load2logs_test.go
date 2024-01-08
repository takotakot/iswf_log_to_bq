package load2logs

import (
	"context"
	"log"
	"testing"

	common "github.com/takotakot/iswf_log_to_bq/common/go"

	"cloud.google.com/go/bigquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockBigqueryClient struct {
	mock.Mock
}

func (m *MockBigqueryClient) Query(q string) common.BigQueryQueryHandle {
	args := m.Called(q)
	return args.Get(0).(common.BigQueryQueryHandle)
}

type MockBigQueryQueryHandle struct {
	mock.Mock
	Parameters []bigquery.QueryParameter
}

type MockBigQueryJobHandle struct {
	mock.Mock
}

func (m *MockBigQueryQueryHandle) Run(ctx context.Context) (j common.BigQueryJobHandle, err error) {
	args := m.Called(ctx)
	return args.Get(0).(common.BigQueryJobHandle), args.Error(1)
}

func (m *MockBigQueryQueryHandle) SetParameters(p []bigquery.QueryParameter) {
	m.Called(p)
	m.Parameters = p
}

type MockBigQueryJobStatusHandle struct {
	mock.Mock
}

func (m *MockBigQueryJobHandle) Wait(ctx context.Context) (common.BigQueryJobStatusHandle, error) {
	args := m.Called(ctx)
	return args.Get(0).(common.BigQueryJobStatusHandle), args.Error(1)
}

func (m *MockBigQueryJobStatusHandle) Err() error {
	args := m.Called()
	return args.Error(0)
}

func TestConstructQuery(t *testing.T) {
	datasetId := "dataset"
	tableId := "table"
	id := "uu-id"

	query := ConstructQuery(datasetId, tableId, id)
	log.Printf("query: %v", query)

	assert.Contains(t, query, "%Y/%m/%d")
	assert.Contains(t, query, id)
	assert.Contains(t, query, datasetId+"."+tableId+"`")
}

func TestLoad2Bq(t *testing.T) {
	ctx := context.Background()

	srcBucketName := "src-bucket"
	srcPath := "test.zip/test.tgz/test.csv"
	srcFileId := "gs://" + srcBucketName + "/" + srcPath
	datasetId := "dataset"
	tableId := "table"

	// mocks
	mockClient := new(MockBigqueryClient)
	mockBigQueryQueryHandle := new(MockBigQueryQueryHandle)
	mockBigQueryJobHandle := new(MockBigQueryJobHandle)
	mockBigQueryJobStatusHandle := new(MockBigQueryJobStatusHandle)

	mockClient.On("Query", mock.Anything).Return(mockBigQueryQueryHandle)
	mockBigQueryQueryHandle.On("Run", mock.Anything).Return(mockBigQueryJobHandle, nil)
	mockBigQueryQueryHandle.On("SetParameters", mock.Anything).Return(nil)
	mockBigQueryJobHandle.On("Wait", mock.Anything).Return(mockBigQueryJobStatusHandle, nil)
	mockBigQueryJobStatusHandle.On("Err", mock.Anything).Return(nil)

	// テストの実行
	err := Load2Bq(ctx, mockClient, srcFileId, datasetId, tableId)

	// 結果の検証
	if !assert.NoError(t, err) {
		t.Errorf("Load2Bq failed: %v", err)
		t.FailNow()
	}

	// モックが期待通りに呼ばれたことを検証
	mockClient.AssertExpectations(t)
	mockBigQueryQueryHandle.AssertExpectations(t)
	mockBigQueryJobHandle.AssertExpectations(t)
	mockBigQueryJobStatusHandle.AssertExpectations(t)
}
