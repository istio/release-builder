package publish

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/ptr"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
)

func NewS3Client() *s3.Client {
	accountId := os.Getenv("CF_ACCOUNT_ID")
	if accountId == "" {
		panic("CF_ACCOUNT_ID environment variable is not set")
	}
	creds := credentials.NewStaticCredentialsProvider(
		os.Getenv("CF_ACCESS_KEY_ID"),
		os.Getenv("CF_SECRET_ACCESS_KEY"),
		"",
	)
	options := s3.Options{
		Region:       "auto",
		BaseEndpoint: ptr.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId)),
		Credentials:  creds,
	}

	return s3.New(options)
}

// FIXME: aliases does nothing.
func ArchiveR2(manifest model.Manifest, bucket string, aliases []string) error {
	ctx := context.Background()
	client := NewS3Client()

	splitbucket := strings.SplitN(bucket, "/", 2)
	bucketName := splitbucket[0]
	objectPrefix := ""
	if len(splitbucket) > 1 {
		objectPrefix = splitbucket[1]
	}

	if err := filepath.Walk(manifest.Directory, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		objName := path.Join(objectPrefix, manifest.Version, strings.TrimPrefix(p, manifest.Directory))
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("failed to open %v: %v", p, err)
		}
		defer f.Close()
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: ptr.String(bucketName),
			Key:    ptr.String(objName),
			Body:   f,
		})
		if err != nil {
			return fmt.Errorf("failed to put object %v: %v", objName, err)
		}
		log.Infof("Wrote %v to r2://%s/%s", p, bucketName, objName)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	return nil
}

func MutateObjectR2(outDir string, client *s3.Client, bkt *string, objectPrefix string, filename string, f func() error) error {
	objName := filepath.Join(objectPrefix, filename)
	outFile := filepath.Join(outDir, filename)
	ctx := context.Background()
	{
		obj, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: bkt,
			Key:    ptr.String(objName),
		})
		if err != nil {
			if !strings.Contains(err.Error(), "NoSuchKey") {
				return fmt.Errorf("failed to fetch object %s: %v", objName, err)
			}
		} else {
			defer obj.Body.Close()
			idx, err := os.Create(outFile)
			if err != nil {
				return fmt.Errorf("failed to create %v: %v", outFile, err)
			}
			defer idx.Close()
			if _, err := idx.ReadFrom(obj.Body); err != nil {
				return fmt.Errorf("failed to read object body: %v", err)
			}
			log.Infof("Wrote %v", outFile)
		}
	}

	// Run our action
	if err := f(); err != nil {
		return fmt.Errorf("action failed: %v", err)
	}

	// Now we want to (try to) write it
	idx, err := os.Open(outFile)
	if err != nil {
		return fmt.Errorf("failed to open %v: %v", outFile, err)
	}
	defer idx.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: bkt,
		Key:    ptr.String(objName),
		Body:   idx,
	})
	if err != nil {
		return fmt.Errorf("failed to write object %s: %v", objName, err)
	}
	return nil
}
