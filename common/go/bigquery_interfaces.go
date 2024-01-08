package common

import (
	"context"

	"cloud.google.com/go/bigquery"
)

// Define abstract interfaces for BigQuery

type BigQueryClient interface {
	Query(q string) BigQueryQueryHandle
}

type BigQueryQueryHandle interface {
	Run(ctx context.Context) (j BigQueryJobHandle, err error)
	SetParameters(p []bigquery.QueryParameter)
}

type BigQueryJobHandle interface {
	Wait(ctx context.Context) (BigQueryJobStatusHandle, error)
}

type BigQueryJobStatusHandle interface {
	Err() error
}

type RealBigQueryClient struct {
	Client *bigquery.Client
}

type RealBigQueryQueryHandle struct {
	query      *bigquery.Query
}

type RealBigQueryJobHandle struct {
	job *bigquery.Job
}

type RealBigQueryJobStatusHandle struct {
	status *bigquery.JobStatus
}

func (r *RealBigQueryClient) Query(q string) BigQueryQueryHandle {
	return &RealBigQueryQueryHandle{query: r.Client.Query(q)}
}

func (r *RealBigQueryQueryHandle) Run(ctx context.Context) (j BigQueryJobHandle, err error) {
	job, err := r.query.Run(ctx)
	return &RealBigQueryJobHandle{job}, err
}

func (r *RealBigQueryQueryHandle) SetParameters(p []bigquery.QueryParameter) {
	r.query.Parameters = p
}

func (s *RealBigQueryJobHandle) Wait(ctx context.Context) (BigQueryJobStatusHandle, error) {
	status, err := s.job.Wait(ctx)
	return &RealBigQueryJobStatusHandle{status}, err
}

func (s *RealBigQueryJobStatusHandle) Err() error {
	return s.status.Err()
}
