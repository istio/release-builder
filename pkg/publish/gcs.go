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

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// GcsArchive publishes the final release archive to the given GCS bucket
func GcsArchive(manifest model.Manifest, bucket string, aliases []string) error {
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
	if err := filepath.Walk(manifest.Directory, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		objName := path.Join(objectPrefix, manifest.Version, strings.TrimPrefix(p, manifest.Directory))
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
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	// Add alias objects. These are basically symlinks/tags for GCS, pointing to the latest version
	for _, alias := range aliases {
		w := bkt.Object(path.Join(objectPrefix, alias)).NewWriter(ctx)
		if _, err := w.Write([]byte(manifest.Version)); err != nil {
			return fmt.Errorf("failed to write alias %v: %v", alias, err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close bucket: %v", err)
		}
		log.Infof("Wrote %v to gs://%s/%s", alias, bucketName, path.Join(objectPrefix, alias))
	}

	return nil
}
