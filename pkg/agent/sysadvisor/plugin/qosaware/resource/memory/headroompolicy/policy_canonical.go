/*
Copyright 2022 The Katalyst Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package headroompolicy

import (
	"fmt"
	"math"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	"github.com/kubewharf/katalyst-core/pkg/agent/sysadvisor/metacache"
	"github.com/kubewharf/katalyst-core/pkg/agent/sysadvisor/plugin/qosaware/resource/helper"
	"github.com/kubewharf/katalyst-core/pkg/agent/sysadvisor/types"
	"github.com/kubewharf/katalyst-core/pkg/metaserver"
)

type PolicyCanonical struct {
	*PolicyBase

	memoryHeadroom float64
	updateStatus   types.UpdateStatus
}

func NewPolicyCanonical(metaCache *metacache.MetaCache, metaServer *metaserver.MetaServer) HeadroomPolicy {
	p := PolicyCanonical{
		PolicyBase: NewPolicyBase(metaCache, metaServer),

		updateStatus: types.UpdateFailed,
	}

	return &p
}

func (p *PolicyCanonical) Update() (err error) {
	defer func() {
		if err != nil {
			p.updateStatus = types.UpdateFailed
		} else {
			p.updateStatus = types.UpdateSucceeded
		}
	}()

	var (
		memoryEstimation float64 = 0
		containerCnt     float64 = 0
		errList          []error
	)

	f := func(podUID string, containerName string, ci *types.ContainerInfo) bool {
		containerEstimation, err := helper.EstimateContainerResourceUsage(ci, v1.ResourceMemory, p.MetaCache)
		if err != nil {
			errList = append(errList, err)
			return true
		}
		klog.Infof("[qosaware-memory-headroom] pod %v container %v estimation %.2e", ci.PodName, containerName, containerEstimation)
		memoryEstimation += containerEstimation
		containerCnt += 1
		return true
	}
	p.MetaCache.RangeContainer(f)
	klog.Infof("[qosaware-memory-headroom] memory requirement estimation: %.2e, #container %v", memoryEstimation, containerCnt)

	p.memoryHeadroom = math.Max(p.Total-p.ReservedForAllocate-memoryEstimation, 0)

	return errors.NewAggregate(errList)
}

func (p *PolicyCanonical) GetHeadroom() (resource.Quantity, error) {
	if p.updateStatus != types.UpdateSucceeded {
		return resource.Quantity{}, fmt.Errorf("last update failed")
	}

	return *resource.NewQuantity(int64(p.memoryHeadroom), resource.DecimalSI), nil
}