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
	"sigs.k8s.io/yaml"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Any charts in any subdirs below the publish root that should also be published
var chartSubtypeDir = []string{
	"samples",
}

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
	client, err := NewGCSClient(ctx)
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
	log.Infof("Using bucket %s and prefix %s", bucketName, objectPrefix)
	bkt := client.Bucket(bucketName)
	log.Infof("Using bucket %v", bkt)

	helmPublishRoot := filepath.Join(manifest.Directory, "helm")

	// Pull down the index, update it, and push it back up.
	// MutateObject ensures there are no races.
	//
	// Note that `helm repo index` will index charts in subdirectories as well, which
	// is desired behavior here - we will have to push them separately however,
	// so the index matches the bucket contents.
	err = MutateObject(helmPublishRoot, bkt, objectPrefix, "index.yaml", func() error {
		dumpIndexFile(filepath.Join(helmPublishRoot, "index.yaml"), "before")
		idxCmd := util.VerboseCommand("helm", "repo", "index", ".",
			"--url", fmt.Sprintf("https://%s.storage.googleapis.com/%s", bucketName, objectPrefix),
			"--merge", "index.yaml")
		idxCmd.Dir = helmPublishRoot
		log.Infof("Running helm repo index with dir %v", idxCmd.Dir)
		if err := idxCmd.Run(); err != nil {
			return fmt.Errorf("index repo: %v", err)
		}
		dumpIndexFile(filepath.Join(helmPublishRoot, "index.yaml"), "after")
		return nil
	})
	if err != nil {
		return fmt.Errorf("helm publish: %v", err)
	}

	// Add extra logging for the actual object in GCS to ensure its written correctly
	liveObject, err := FetchObject(bkt, objectPrefix, "index.yaml")
	if err != nil {
		log.Warnf("failed to get live index.yaml: %v", err)
	} else {
		dumpIndex(liveObject, "live")
	}

	// Now push all the packaged charts in the helm root directory up
	if err := publishHelmBucket(ctx, helmPublishRoot, objectPrefix, bucketName, bkt); err != nil {
		return err
	}

	// For any packaged charts in "chart subtype" subdirectories ("samples" etc), push those up
	for _, chartType := range chartSubtypeDir {
		if err := publishHelmBucket(ctx, filepath.Join(helmPublishRoot, chartType), path.Join(objectPrefix, chartType), bucketName, bkt); err != nil {
			return err
		}
	}

	return nil
}

func publishHelmBucket(ctx context.Context, packagedChartOutputDir, publishPrefix, bName string, bkt *storage.BucketHandle) error {
	dirInfo, err := os.ReadDir(packagedChartOutputDir)
	if err != nil {
		return err
	}
	for _, f := range dirInfo {
		if filepath.Ext(f.Name()) != ".tgz" {
			log.Infof("skipping %v", f.Name())
			continue
		}
		objName := path.Join(publishPrefix, f.Name())
		obj := bkt.Object(objName)
		w := obj.NewWriter(ctx)
		f, err := os.Open(filepath.Join(packagedChartOutputDir, f.Name()))
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
		log.Infof("Wrote %v to gs://%s/%s", f.Name(), bName, objName)
	}

	return nil
}

type helmChart struct {
	AppVersion string `json:"appVersion"`
}

type helmIndex struct {
	Entries map[string][]helmChart `json:"entries"`
}

// dumpIndexFile outputs a summary of a helm index.yaml file, for debugging.
func dumpIndexFile(fpath string, context string) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		log.Errorf("failed to read %v: %v", fpath, err)
		return
	}
	dumpIndex(data, context)
}

// dumpIndex outputs a summary of a helm index.yaml contents, for debugging.
func dumpIndex(data []byte, context string) {
	idx := &helmIndex{}
	if err := yaml.Unmarshal(data, idx); err != nil {
		log.Errorf("failed to unmarshal %v: %v", string(data), err)
		return
	}
	versions := []string{}
	// Only look at base since all charts *should* be the same set of versions.
	for _, hc := range idx.Entries["base"] {
		versions = append(versions, hc.AppVersion)
	}
	log.Infof("index.yaml contents %v: %v", context, versions)
}

func publishHelmOCI(manifest model.Manifest, hub string) error {
	helmPublishRoot := filepath.Join(manifest.Directory, "helm")

	// Now push all the packaged charts in the helm root directory up
	if err := pushChartsInDirOCI(helmPublishRoot, hub); err != nil {
		return err
	}

	// For any packaged charts in "chart subtype" subdirectories ("samples" etc), push those up
	for _, chartType := range chartSubtypeDir {
		if err := pushChartsInDirOCI(filepath.Join(helmPublishRoot, chartType), path.Join(hub, chartType)); err != nil {
			return err
		}
	}

	return nil
}

func pushChartsInDirOCI(packagedChartOutputDir, hub string) error {
	dirInfo, err := os.ReadDir(packagedChartOutputDir)
	if err != nil {
		return err
	}
	// Publish as OCI artifacts
	for _, f := range dirInfo {
		if filepath.Ext(f.Name()) != ".tgz" {
			continue
		}
		name := filepath.Join(packagedChartOutputDir, f.Name())
		if err := util.VerboseCommand("helm", "push", name, "oci://"+hub).Run(); err != nil {
			return fmt.Errorf("failed to load docker image %v: %v", f.Name(), err)
		}
	}
	return nil
}
