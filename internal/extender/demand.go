// Copyright (c) 2019 Palantir Technologies. All rights reserved.
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

package extender

import (
	"context"
	"encoding/json"

	demandapi "github.com/palantir/k8s-spark-scheduler-lib/pkg/apis/scaler/v1alpha2"
	"github.com/palantir/k8s-spark-scheduler-lib/pkg/resources"
	"github.com/palantir/k8s-spark-scheduler/internal"
	"github.com/palantir/k8s-spark-scheduler/internal/cache"
	"github.com/palantir/k8s-spark-scheduler/internal/common"
	"github.com/palantir/k8s-spark-scheduler/internal/common/utils"
	"github.com/palantir/k8s-spark-scheduler/internal/events"
	werror "github.com/palantir/witchcraft-go-error"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

const (
	podDemandCreated v1.PodConditionType = "PodDemandCreated"
)

var (
	demandCreatedCondition = &v1.PodCondition{
		Type:   podDemandCreated,
		Status: v1.ConditionTrue,
	}
)

// TODO: should patch instead of put to avoid conflicts
func (s *SparkSchedulerExtender) updatePodStatus(ctx context.Context, pod *v1.Pod, _ *v1.PodCondition) {
	if !podutil.UpdatePodCondition(&pod.Status, demandCreatedCondition) {
		svc1log.FromContext(ctx).Info("pod condition for demand creation already exist")
		return
	}
	_, err := s.coreClient.Pods(pod.Namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		svc1log.FromContext(ctx).Warn("pod condition update failed", svc1log.SafeParam("reason", err.Error()))
	}
}

func (s *SparkSchedulerExtender) createDemandForExecutorInAnyZone(ctx context.Context, executorPod *v1.Pod, executorResources *resources.Resources) {
	s.createDemandForExecutorInSpecificZone(ctx, executorPod, executorResources, nil)
}

func (s *SparkSchedulerExtender) createDemandForExecutorInSpecificZone(ctx context.Context, executorPod *v1.Pod, executorResources *resources.Resources, zone *demandapi.Zone) {
	if !s.demands.CRDExists() {
		return
	}
	units := []demandapi.DemandUnit{
		{
			Count: 1,
			Resources: demandapi.ResourceList{
				demandapi.ResourceCPU:       executorResources.CPU,
				demandapi.ResourceMemory:    executorResources.Memory,
				demandapi.ResourceNvidiaGPU: executorResources.NvidiaGPU,
			},
			PodNamesByNamespace: map[string][]string{
				executorPod.Namespace: {executorPod.Name},
			},
		},
	}
	s.createDemand(ctx, executorPod, units, zone)
}

func (s *SparkSchedulerExtender) createDemandForApplicationInAnyZone(ctx context.Context, driverPod *v1.Pod, applicationResources *sparkApplicationResources) {
	if !s.demands.CRDExists() {
		return
	}
	s.createDemand(ctx, driverPod, demandResourcesForApplication(driverPod, applicationResources), nil)
}

func (s *SparkSchedulerExtender) createDemand(ctx context.Context, pod *v1.Pod, demandUnits []demandapi.DemandUnit, zone *demandapi.Zone) {
	instanceGroup, ok := internal.FindInstanceGroupFromPodSpec(pod.Spec, s.instanceGroupLabel)
	if !ok {
		svc1log.FromContext(ctx).Error("No instanceGroup label exists. Cannot map to InstanceGroup. Skipping demand object",
			svc1log.SafeParam("expectedLabel", s.instanceGroupLabel))
		return
	}

	newDemand, err := s.newDemand(pod, instanceGroup, demandUnits, zone)
	if err != nil {
		svc1log.FromContext(ctx).Error("failed to construct demand object", svc1log.Stacktrace(err))
		return
	}
	err = s.doCreateDemand(ctx, newDemand)
	if err != nil {
		svc1log.FromContext(ctx).Error("failed to create demand", svc1log.Stacktrace(err))
		return
	}
	go s.updatePodStatus(ctx, pod, demandCreatedCondition)
}

func (s *SparkSchedulerExtender) doCreateDemand(ctx context.Context, newDemand *demandapi.Demand) error {
	demandObjectBytes, err := json.Marshal(newDemand)
	if err != nil {
		return werror.Wrap(err, "failed to marshal demand object")
	}
	svc1log.FromContext(ctx).Info("Creating demand object", svc1log.SafeParams(internal.DemandSafeParamsFromObj(newDemand)), svc1log.SafeParam("demandObjectBytes", string(demandObjectBytes)))
	err = s.demands.Create(newDemand)
	if err != nil {
		_, ok := s.demands.Get(newDemand.Namespace, newDemand.Name)
		if ok {
			svc1log.FromContext(ctx).Info("demand object already exists for pod so no action will be taken")
			return nil
		}
	}
	events.EmitDemandCreated(ctx, newDemand)
	return err
}

func (s *SparkSchedulerExtender) removeDemandIfExists(ctx context.Context, pod *v1.Pod) {
	DeleteDemandIfExists(ctx, s.demands, pod, "SparkSchedulerExtender")
}

// DeleteDemandIfExists removes a demand object if it exists, and emits an event tagged by the source of the deletion
func DeleteDemandIfExists(ctx context.Context, cache *cache.SafeDemandCache, pod *v1.Pod, source string) {
	if !cache.CRDExists() {
		return
	}
	demandName := utils.DemandName(pod)
	if demand, ok := cache.Get(pod.Namespace, demandName); ok {
		// there is no harm in the demand being deleted elsewhere in between the two calls.
		cache.Delete(pod.Namespace, demandName)
		svc1log.FromContext(ctx).Info("Removed demand object for pod", svc1log.SafeParams(internal.DemandSafeParams(demandName, pod.Namespace)))
		events.EmitDemandDeleted(ctx, demand, source)
	}
}

func (s *SparkSchedulerExtender) newDemand(pod *v1.Pod, instanceGroup string, units []demandapi.DemandUnit, zone *demandapi.Zone) (*demandapi.Demand, error) {
	appID, ok := pod.Labels[common.SparkAppIDLabel]
	if !ok {
		return nil, werror.Error("pod did not contain expected label for AppID", werror.SafeParam("expectedLabel", common.SparkAppIDLabel))
	}
	demandName := utils.DemandName(pod)
	return &demandapi.Demand{
		ObjectMeta: metav1.ObjectMeta{
			Name:      demandName,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				common.SparkAppIDLabel: appID,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pod, podGroupVersionKind),
			},
		},
		Spec: demandapi.DemandSpec{
			InstanceGroup:               instanceGroup,
			Units:                       units,
			EnforceSingleZoneScheduling: s.binpacker.IsSingleAz,
			Zone:                        zone,
		},
	}, nil
}

func demandResourcesForApplication(driverPod *v1.Pod, applicationResources *sparkApplicationResources) []demandapi.DemandUnit {
	demandUnits := []demandapi.DemandUnit{
		{
			Count: 1,
			Resources: demandapi.ResourceList{
				demandapi.ResourceCPU:       applicationResources.driverResources.CPU,
				demandapi.ResourceMemory:    applicationResources.driverResources.Memory,
				demandapi.ResourceNvidiaGPU: applicationResources.driverResources.NvidiaGPU,
			},
			// By specifying the pod driver pod here, we don't duplicate the resources of the pod with the created demand
			PodNamesByNamespace: map[string][]string{
				driverPod.Namespace: {driverPod.Name},
			},
		},
	}
	if applicationResources.minExecutorCount > 0 {
		demandUnits = append(demandUnits, demandapi.DemandUnit{
			Count: applicationResources.minExecutorCount,
			Resources: demandapi.ResourceList{
				demandapi.ResourceCPU:       applicationResources.executorResources.CPU,
				demandapi.ResourceMemory:    applicationResources.executorResources.Memory,
				demandapi.ResourceNvidiaGPU: applicationResources.executorResources.NvidiaGPU,
			},
		})
	}
	return demandUnits
}
