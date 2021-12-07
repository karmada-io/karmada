/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha4

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Cluster)

	return Convert_v1alpha4_Cluster_To_v1beta1_Cluster(src, dst, nil)
}

func (dst *Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Cluster)

	return Convert_v1beta1_Cluster_To_v1alpha4_Cluster(src, dst, nil)
}

func (src *ClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.ClusterList)

	return Convert_v1alpha4_ClusterList_To_v1beta1_ClusterList(src, dst, nil)
}

func (dst *ClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.ClusterList)

	return Convert_v1beta1_ClusterList_To_v1alpha4_ClusterList(src, dst, nil)
}

func (src *ClusterClass) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.ClusterClass)

	return Convert_v1alpha4_ClusterClass_To_v1beta1_ClusterClass(src, dst, nil)
}

func (dst *ClusterClass) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.ClusterClass)

	return Convert_v1beta1_ClusterClass_To_v1alpha4_ClusterClass(src, dst, nil)
}

func (src *ClusterClassList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.ClusterClassList)

	return Convert_v1alpha4_ClusterClassList_To_v1beta1_ClusterClassList(src, dst, nil)
}

func (dst *ClusterClassList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.ClusterClassList)

	return Convert_v1beta1_ClusterClassList_To_v1alpha4_ClusterClassList(src, dst, nil)
}

func (src *Machine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Machine)

	return Convert_v1alpha4_Machine_To_v1beta1_Machine(src, dst, nil)
}

func (dst *Machine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Machine)

	return Convert_v1beta1_Machine_To_v1alpha4_Machine(src, dst, nil)
}

func (src *MachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineList)

	return Convert_v1alpha4_MachineList_To_v1beta1_MachineList(src, dst, nil)
}

func (dst *MachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineList)

	return Convert_v1beta1_MachineList_To_v1alpha4_MachineList(src, dst, nil)
}

func (src *MachineSet) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineSet)

	return Convert_v1alpha4_MachineSet_To_v1beta1_MachineSet(src, dst, nil)
}

func (dst *MachineSet) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineSet)

	return Convert_v1beta1_MachineSet_To_v1alpha4_MachineSet(src, dst, nil)
}

func (src *MachineSetList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineSetList)

	return Convert_v1alpha4_MachineSetList_To_v1beta1_MachineSetList(src, dst, nil)
}

func (dst *MachineSetList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineSetList)

	return Convert_v1beta1_MachineSetList_To_v1alpha4_MachineSetList(src, dst, nil)
}

func (src *MachineDeployment) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineDeployment)

	return Convert_v1alpha4_MachineDeployment_To_v1beta1_MachineDeployment(src, dst, nil)
}

func (dst *MachineDeployment) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineDeployment)

	return Convert_v1beta1_MachineDeployment_To_v1alpha4_MachineDeployment(src, dst, nil)
}

func (src *MachineDeploymentList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineDeploymentList)

	return Convert_v1alpha4_MachineDeploymentList_To_v1beta1_MachineDeploymentList(src, dst, nil)
}

func (dst *MachineDeploymentList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineDeploymentList)

	return Convert_v1beta1_MachineDeploymentList_To_v1alpha4_MachineDeploymentList(src, dst, nil)
}

func (src *MachineHealthCheck) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineHealthCheck)

	return Convert_v1alpha4_MachineHealthCheck_To_v1beta1_MachineHealthCheck(src, dst, nil)
}

func (dst *MachineHealthCheck) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineHealthCheck)

	return Convert_v1beta1_MachineHealthCheck_To_v1alpha4_MachineHealthCheck(src, dst, nil)
}

func (src *MachineHealthCheckList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MachineHealthCheckList)

	return Convert_v1alpha4_MachineHealthCheckList_To_v1beta1_MachineHealthCheckList(src, dst, nil)
}

func (dst *MachineHealthCheckList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MachineHealthCheckList)

	return Convert_v1beta1_MachineHealthCheckList_To_v1alpha4_MachineHealthCheckList(src, dst, nil)
}

func Convert_v1alpha4_MachineStatus_To_v1beta1_MachineStatus(in *MachineStatus, out *v1beta1.MachineStatus, s apiconversion.Scope) error {
	// Status.version has been removed in v1beta1, thus requiring custom conversion function. the information will be dropped.
	return autoConvert_v1alpha4_MachineStatus_To_v1beta1_MachineStatus(in, out, s)
}
