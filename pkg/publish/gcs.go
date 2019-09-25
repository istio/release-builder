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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/howardjohn/istio-release/pkg/model"
	"istio.io/pkg/log"
)

// GcsArchive publishes the final release archive to the given GCS bucket
func GcsArchive(manifest model.Manifest, bucket string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
		return err
	}

	// Allow the caller to pass a reference like bucket/folder/subfolder, but split this to
	// bucket, and folder/subfolder prefix
	splitbucket := strings.SplitN(bucket, "/", 2)
	bucketName := splitbucket[0]
	objectPrefix := ""
	if len(splitbucket) > 1 {
		objectPrefix = splitbucket[1]
	}
	bkt := client.Bucket(bucketName)
	if err := filepath.Walk(manifest.OutDir(), func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		objName := path.Join(objectPrefix, manifest.Version, strings.TrimPrefix(p, manifest.OutDir()))
		obj := bkt.Object(objName)
		w := obj.NewWriter(ctx)
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("failed to open %v: %v", p, err)
		}
		r := bufio.NewReader(f)
		if _, err := io.Copy(w, r); err != nil {
			return fmt.Errorf("failed writing %v: %v", p, err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close bucket: %v", err)
		}
		log.Infof("Wrote %v to gs://%s/%s", p, bucketName, objName)
		return nil

	}); err != nil {
		return err
	}
	_ = bkt
	return nil
}
