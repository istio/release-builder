package publish

import (
	"fmt"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

func GcsArchive(manifest model.Manifest, bucket string) error {
	// TODO use golang libraries
	// A bit painful since we cannot just copy the directory it seems, but must do each file
	if err := util.VerboseCommand("gsutil", "-m", "cp", "-r", manifest.OutDir()+"/*", bucket+"/"+manifest.Version+"/").Run(); err != nil {
		return fmt.Errorf("failed to write to gcs: %v", err)
	}
	return nil
}
