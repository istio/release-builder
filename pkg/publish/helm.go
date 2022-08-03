// Copyright Istio Authors
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
	"istio.io/release-builder/pkg/util"
)

// Helm publishes charts to the given GCS bucket
func Helm(manifest model.Manifest, bucket string, hub string) error {
	if bucket != "" {
		if err := publishHelmIndex(manifest, bucket); err != nil {
			return err
		}
	}

	if hub != "" {
		if err := publishHelmOCI(manifest, hub); err != nil {
			return err
		}
	}

	return nil
}

func publishHelmIndex(manifest model.Manifest, bucket string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
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

	helmDir := filepath.Join(manifest.Directory, "helm")

	// Pull down the index, update it, and push it back up.
	// MutateObject ensures there are no races.
	err = MutateObject(helmDir, bkt, objectPrefix, "index.yaml", func() error {
		idxCmd := util.VerboseCommand("helm", "repo", "index", ".",
			"--url", fmt.Sprintf("https://%s.storage.googleapis.com/%s", bucketName, objectPrefix),
			"--merge", "index.yaml")
		idxCmd.Dir = helmDir
		log.Infof("Running helm repo index with dir %v", idxCmd.Dir)
		if err := idxCmd.Run(); err != nil {
			return fmt.Errorf("index repo: %v", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("helm publish: %v", err)
	}

	// Now push all our charts up
	dirInfo, err := os.ReadDir(helmDir)
	if err != nil {
		return err
	}
	for _, f := range dirInfo {
		if filepath.Ext(f.Name()) != ".tgz" {
			log.Infof("skipping %v", f.Name())
			continue
		}
		objName := path.Join(objectPrefix, f.Name())
		obj := bkt.Object(objName)
		w := obj.NewWriter(ctx)
		f, err := os.Open(filepath.Join(helmDir, f.Name()))
		if err != nil {
			return fmt.Errorf("failed to open %v: %v", f.Name(), err)
		}
		r := bufio.NewReader(f)
		if _, err := io.Copy(w, r); err != nil {
			return fmt.Errorf("failed writing %v: %v", f.Name(), err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close bucket: %v", err)
		}
		log.Infof("Wrote %v to gs://%s/%s", f.Name(), bucketName, objName)
	}
	return nil
}

func publishHelmOCI(manifest model.Manifest, hub string) error {
	helmDir := filepath.Join(manifest.Directory, "helm")
	dirInfo, err := os.ReadDir(helmDir)
	if err != nil {
		return err
	}
	// Publish as OCI artifacts
	for _, f := range dirInfo {
		if filepath.Ext(f.Name()) != ".tgz" {
			continue
		}
		name := filepath.Join(helmDir, f.Name())
		if err := util.VerboseCommand("helm", "push", name, "oci://"+hub).Run(); err != nil {
			return fmt.Errorf("failed to load docker image %v: %v", f.Name(), err)
		}
	}
	return nil
}
