package storage

import (
	"github.com/kubeshop/testkube/pkg/api/v1/testkube"
	"github.com/minio/minio-go/v7"
)

type Client interface {
	CreateBucket(bucket string) error
	DeleteBucket(bucket string, force bool) error
	ListBuckets() ([]string, error)
	ListFiles(bucket string) ([]testkube.Artifact, error)
	SaveFile(bucket, filePath string) error
	DownloadFile(bucket, file string) (*minio.Object, error)
}
