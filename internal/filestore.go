package internal

import (
	"context"
	"io"
	"real-time-chat/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileStore struct {
	client *minio.Client
	bucket string
}

func NewFileStore(ctx context.Context, cfg *config.Config) (*FileStore, error) {

	newFS, err := minio.New(cfg.MinioConn, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioUsername, cfg.MinioPassword, ""),
		Secure: false,
	})

	if err != nil {
		return nil, err
	}

	exists, err := newFS.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		err = newFS.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}

		policy := `{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::` + cfg.MinioBucket + `/*"]
			}]
		}`

		newFS.SetBucketPolicy(ctx, cfg.MinioBucket, policy)
	}

	return &FileStore{
		client: newFS,
		bucket: cfg.MinioBucket,
	}, nil
}

func (fs *FileStore) Upload(ctx context.Context, name string, reader io.Reader, size int64) (string, error) {
	_, err := fs.client.PutObject(ctx, fs.bucket, name, reader, size, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	res := fs.client.EndpointURL().String() + "/" + fs.bucket + "/" + name

	return res, nil
}
