package unzip

import (
	"context"
	"io"
	"log"
	"os"
	"path"

	"testing"

	common "github.com/takotakot/iswf_log_to_bq/common/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) Bucket(name string) common.BucketHandle {
	args := m.Called(name)
	return args.Get(0).(common.BucketHandle)
}

type MockBucketHandle struct {
	mock.Mock
	Name string
}

func (m *MockBucketHandle) Object(name string) common.ObjectHandle {
	args := m.Called(name)
	args.Get(0).(*MockObjectHandle).Name = name
	return args.Get(0).(common.ObjectHandle)
}

type MockObjectHandle struct {
	mock.Mock
	Name string
}

type MockReadCloser struct {
	mock.Mock
	data []byte // 読み込むデータ
	pos  int    // 現在の読み込み位置
}

type MockObjectWriter struct {
	mock.Mock
	WrittenData []byte
}

func (m *MockReadCloser) Read(p []byte) (int, error) {
	m.Called(p)
	if m.pos >= len(m.data) {
		// データの終わりに達した
		return 0, io.EOF
	}
	n := copy(p, m.data[m.pos:]) // バッファにデータをコピー
	m.pos += n
	return n, nil
}

func (m *MockReadCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockObjectWriter) Write(p []byte) (int, error) {
	m.Called(p)
	log.Printf("MockObjectWriter.Write: %v", p)
	m.WrittenData = append(m.WrittenData, p...)
	return len(p), nil
}

func (m *MockObjectWriter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockObjectHandle) NewReader(ctx context.Context) (io.ReadCloser, error) {
	args := m.Called(ctx)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockObjectHandle) NewWriter(ctx context.Context) io.WriteCloser {
	args := m.Called(ctx)
	return args.Get(0).(io.WriteCloser)
}

func readFileContent(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// TestExtractAndUploadはExtractAndUpload関数のユニットテスト
func TestExtractAndUpload(t *testing.T) {
	ctx := context.Background()

	fileData, err := readFileContent("../test_files/test.zip")
	if err != nil {
		t.Fatal(err)
	}

	destFileData, err := readFileContent("../test_files/test.tgz")
	if err != nil {
		t.Fatal(err)
	}

	// テスト用の zip ファイルとバケット名を設定
	srcBucketName := "src-bucket"
	srcPath := "test.zip"
	srcSize := int64(len(fileData))
	destBucketName := "dest-bucket"
	contentFileName := "test.tgz"
	destPath := path.Join(srcPath, contentFileName)

	// mocks
	// messageSenderのモック実装
	messageSender := func(msgData common.PubSubMessageData) error {
		// 期待されるメッセージデータ
		expectedMsgData := common.PubSubMessageData{
			Bucket:   destBucketName,
			FilePath: path.Join(srcPath, contentFileName),
		}
		assert.Equal(t, expectedMsgData, msgData, "Message data does not match the expected data")

		return nil
	}

	mockClient := new(MockStorageClient)
	mockSrcBucketHandle := &MockBucketHandle{
		Name: srcBucketName,
	}
	mockDestBucketHandle := &MockBucketHandle{
		Name: destBucketName,
	}
	mockSrcObjectHandle := new(MockObjectHandle)
	mockDestObjectHandle := new(MockObjectHandle)
	mockReadCloser := &MockReadCloser{
		data: fileData,
	}
	mockObjectWriter := new(MockObjectWriter)

	mockClient.On("Bucket", srcBucketName).Return(mockSrcBucketHandle)
	mockClient.On("Bucket", destBucketName).Return(mockDestBucketHandle)
	mockSrcBucketHandle.On("Object", srcPath).Return(mockSrcObjectHandle)
	mockDestBucketHandle.On("Object", destPath).Return(mockDestObjectHandle)

	mockSrcObjectHandle.On("NewReader", mock.Anything).Return(mockReadCloser, nil)
	mockDestObjectHandle.On("NewWriter", mock.Anything).Return(mockObjectWriter)
	mockReadCloser.On("Read", mock.Anything)
	mockReadCloser.On("Close").Return(nil)
	mockObjectWriter.On("Write", mock.Anything)
	mockObjectWriter.On("Close").Return(nil)

	// テストの実行
	err = ExtractAndUpload(ctx, mockClient, srcBucketName, srcPath, srcSize, destBucketName, messageSender)
	if err != nil {
		t.Errorf("ExtractAndUpload failed: %v", err)
	}

	// 結果の検証
	assert.Equal(t, destFileData, mockObjectWriter.WrittenData, "Written data does not match the test.tar.gz file")

	// モックが期待通りに呼ばれたことを検証
	mockClient.AssertExpectations(t)
	mockSrcBucketHandle.AssertExpectations(t)
	mockDestBucketHandle.AssertExpectations(t)
	mockSrcObjectHandle.AssertExpectations(t)
	mockDestObjectHandle.AssertExpectations(t)
	mockReadCloser.AssertExpectations(t)
	mockObjectWriter.AssertExpectations(t)
}
