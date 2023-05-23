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

package validate

import (
	"fmt"

	"github.com/spf13/cobra"

	"istio.io/istio/pkg/log"
)

var (
	flags = struct {
		release string
	}{}

	validateCmd = &cobra.Command{
		Use:          "validate",
		Short:        "Validates a release of Istio",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			passed, info, failed := CheckRelease(flags.release)
			for _, pass := range passed {
				log.Infof("Check passed: %v", pass)
			}
			for _, fail := range failed {
				log.Infof("Check failed: %v", fail)
			}
			log.Infof("Debug output:\n%v", info)
			if len(failed) > 0 {
				return fmt.Errorf("release validation FAILED")
			}
			log.Info("Release validation PASSED")
			return nil
		},
	}
)

func init() {
	validateCmd.PersistentFlags().StringVar(&flags.release, "release", flags.release,
		"The release to validate.")
}

func GetValidateCommand() *cobra.Command {
	return validateCmd
}
