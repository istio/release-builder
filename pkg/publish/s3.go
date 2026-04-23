// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package publish

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/ptr"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
)

func NewS3Client() (*s3.Client, error) {
	options := []func(*config.LoadOptions) error{}

	if flags.s3BaseEndpoint == "" {
		log.Warnf("No custom S3 endpoint provided, using AWS S3 by default")
	} else {
		options = append(options, config.WithBaseEndpoint(flags.s3BaseEndpoint))
	}

	// This will read
	// * AWS_REGION
	// * AWS_ACCESS_KEY_ID
    // * AWS_SECRET_ACCESS_KEY
    // * AWS_SESSION_TOKEN
	// from the environment
	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg), nil
}

func ArchiveS3(manifest model.Manifest, bucket string, aliases []string) error {
	ctx := context.Background()
	client, err := NewS3Client()
	if err != nil {
		return err
	}

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

// MutateObjectS3 pulls a file from S3, mutates it, then pushes it back up.
// It uses ETag-based conditional writes to retry if the object was modified concurrently.
func MutateObjectS3(outDir string, client *s3.Client, bkt *string, objectPrefix string, filename string, f func() error) error {
	for i := 0; i < 10; i++ {
		err := mutateObjectS3Inner(outDir, client, bkt, objectPrefix, filename, f)
		if err == ErrIndexOutOfDate {
			log.Warnf("Write conflict, trying again")
			continue
		}
		return err
	}
	return fmt.Errorf("max conflicts attempted")
}

func mutateObjectS3Inner(outDir string, client *s3.Client, bkt *string, objectPrefix string, filename string, f func() error) error {
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
