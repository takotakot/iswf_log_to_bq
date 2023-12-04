package common

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

// Define abstract interfaces for Cloud Storage

// The StorageClient interface is defined for the *storage.Client type.
type StorageClient interface {
	Bucket(name string) BucketHandle
}

type BucketHandle interface {
	Object(name string) ObjectHandle
}

type ObjectHandle interface {
	NewReader(ctx context.Context) (io.ReadCloser, error)
	NewWriter(ctx context.Context) io.WriteCloser
}

type ObjectWriter interface {
	Write(p []byte) (n int, err error)
	Close() error
}

// Map the abstract interfaces to the real implementation

type RealStorageClient struct {
	Client *storage.Client
}

func (r *RealStorageClient) Bucket(name string) BucketHandle {
	return &RealStorageBucketHandle{bucket: r.Client.Bucket(name)}
}

type RealStorageBucketHandle struct {
	bucket *storage.BucketHandle
}

func (rbh *RealStorageBucketHandle) Object(name string) ObjectHandle {
	return &RealStorageObjectHandle{object: rbh.bucket.Object(name)}
}

type RealStorageObjectHandle struct {
	object *storage.ObjectHandle
}

func (roh *RealStorageObjectHandle) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return roh.object.NewReader(ctx)
}

func (roh *RealStorageObjectHandle) NewWriter(ctx context.Context) io.WriteCloser {
	return roh.object.NewWriter(ctx)
}
