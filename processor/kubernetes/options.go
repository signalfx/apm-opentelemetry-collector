// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import "fmt"

// Option ...
type Option func(*kubernetesprocessor) error

// WithMetadataField ...
func WithMetadataField(field string) Option {
	return func(p *kubernetesprocessor) error {
		switch field {
		case "podName":
			p.rules.PodName = true
		case "startTime":
			p.rules.StartTime = true
		default:
			fmt.Println("****** dksjakdjaksjdkajskdj")
			return fmt.Errorf("\"%s\" is not a supported metadat field", field)
		}
		return nil
	}
}