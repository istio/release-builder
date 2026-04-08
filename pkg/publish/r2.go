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

	// Add alias objects that contain the version string, pointing to the latest version
	for _, alias := range aliases {
		aliasKey := path.Join(objectPrefix, alias)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      ptr.String(bucketName),
			Key:         ptr.String(aliasKey),
			Body:        strings.NewReader(manifest.Version),
			ContentType: ptr.String("text/plain"),
		})
		if err != nil {
			return fmt.Errorf("failed to write alias %v: %v", alias, err)
		}
		log.Infof("Wrote alias %v to r2://%s/%s", alias, bucketName, aliasKey)
	}

	return nil
}

// MutateObjectR2 pulls a file from R2, mutates it, then pushes it back up.
// It uses ETag-based conditional writes to retry if the object was modified concurrently.
func MutateObjectR2(outDir string, client *s3.Client, bkt *string, objectPrefix string, filename string, f func() error) error {
	for i := 0; i < 10; i++ {
		err := mutateObjectR2Inner(outDir, client, bkt, objectPrefix, filename, f)
		if err == ErrIndexOutOfDate {
			log.Warnf("Write conflict, trying again")
			continue
		}
		return err
	}
	return fmt.Errorf("max conflicts attempted")
}

func mutateObjectR2Inner(outDir string, client *s3.Client, bkt *string, objectPrefix string, filename string, f func() error) error {
	objName := filepath.Join(objectPrefix, filename)
	outFile := filepath.Join(outDir, filename)
	ctx := context.Background()

	// Track the ETag for conditional writes
	var etag *string

	obj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: bkt,
		Key:    ptr.String(objName),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "NoSuchKey") {
			return fmt.Errorf("failed to fetch object %s: %v", objName, err)
		}
		log.Warnf("existing file %v does not exist", filename)
	} else {
		etag = obj.ETag
		log.Infof("Object %v currently has ETag %s", objName, *etag)

		idx, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("failed to create %v: %v", outFile, err)
		}
		if _, err := idx.ReadFrom(obj.Body); err != nil {
			idx.Close()
			obj.Body.Close()
			return fmt.Errorf("failed to read object body: %v", err)
		}
		idx.Close()
		obj.Body.Close()
		log.Infof("Wrote %v", outFile)
	}

	// Run our action
	if err := f(); err != nil {
		return fmt.Errorf("action failed: %v", err)
	}

	// Now write it back with a conditional put
	idx, err := os.Open(outFile)
	if err != nil {
		return fmt.Errorf("failed to open %v: %v", outFile, err)
	}
	defer idx.Close()

	putInput := &s3.PutObjectInput{
		Bucket:       bkt,
		Key:          ptr.String(objName),
		Body:         idx,
		ContentType:  ptr.String("text/yaml"),
		CacheControl: ptr.String("no-cache, max-age=0, no-transform"),
	}
	// If the object existed, use If-Match to ensure it hasn't been modified since we read it.
	if etag != nil {
		putInput.IfMatch = etag
	}

	_, err = client.PutObject(ctx, putInput)
	if err != nil {
		// A 412 Precondition Failed means someone else wrote to the object since we read it.
		if strings.Contains(err.Error(), "PreconditionFailed") || strings.Contains(err.Error(), "412") {
			return ErrIndexOutOfDate
		}
		return fmt.Errorf("failed to write object %s: %v", objName, err)
	}
	return nil
}
