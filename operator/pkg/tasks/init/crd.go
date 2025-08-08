/*
Copyright 2023 The Karmada Authors.

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

package tasks

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

var (
	crdsFileSuffix = "crds.tar.gz"
	crdPathSuffix  = "crds"
)

// NewPrepareCrdsTask init a prepare-crds task
func NewPrepareCrdsTask() workflow.Task {
	return workflow.Task{
		Name:        "prepare-crds",
		Run:         runPrepareCrds,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "download-crds",
				Skip: skipCrdsDownload,
				Run:  runCrdsDownload,
			},
			{
				Name: "Unpack",
				Run:  runUnpack,
			},
			{
				Name: "post-check",
				Run:  postCheck,
			},
		},
	}
}

func runPrepareCrds(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("prepare-crds task invoked with an invalid data struct")
	}

	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return fmt.Errorf("[prepare-crds] failed to get CRD dir, err: %w", err)
	}

	klog.V(4).InfoS("[prepare-crds] Running prepare-crds task", "karmada", klog.KObj(data))
	klog.V(2).InfoS("[prepare-crds] Using crd folder", "folder", crdsDir, "karmada", klog.KObj(data))

	return nil
}

func skipCrdsDownload(r workflow.RunData) (bool, error) {
	data, ok := r.(InitData)
	if !ok {
		return false, errors.New("prepare-crds task invoked with an invalid data struct")
	}

	crdTarball := data.CrdTarball()
	if crdTarball.CRDDownloadPolicy != nil && *crdTarball.CRDDownloadPolicy == operatorv1alpha1.DownloadAlways {
		klog.V(2).InfoS("[skipCrdsDownload] CrdDownloadPolicy is 'Always', skipping cache check")
		return false, nil
	}

	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return false, fmt.Errorf("[skipCrdsDownload] failed to get CRD dir, err: %w", err)
	}

	if exist, err := util.PathExists(crdsDir); !exist || err != nil {
		return false, err
	}

	if !existCrdsTar(crdsDir) {
		return false, nil
	}

	return true, nil
}

func runCrdsDownload(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("download-crds task invoked with an invalid data struct")
	}

	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return fmt.Errorf("[download-crds] failed to get CRD dir, err: %w", err)
	}
	crdsTarPath := path.Join(crdsDir, crdsFileSuffix)

	exist, err := util.PathExists(crdsDir)
	if err != nil {
		return err
	}

	if exist {
		if err := os.RemoveAll(crdsDir); err != nil {
			return fmt.Errorf("failed to delete CRDs directory, err: %w", err)
		}
	}

	if err := os.MkdirAll(crdsDir, 0700); err != nil {
		return fmt.Errorf("failed to create CRDs directory, err: %w", err)
	}

	crdTarball := data.CrdTarball()
	if err := util.DownloadFile(crdTarball.HTTPSource.URL, crdsTarPath, crdTarball.HTTPSource.Proxy); err != nil {
		return fmt.Errorf("failed to download CRD tar, err: %w", err)
	}

	klog.V(2).InfoS("[download-crds] Successfully downloaded crd package", "karmada", klog.KObj(data))
	return nil
}

func runUnpack(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("unpack task invoked with an invalid data struct")
	}

	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return fmt.Errorf("[unpack] failed to get CRD dir, err: %w", err)
	}
	crdsTarPath := path.Join(crdsDir, crdsFileSuffix)
	crdsPath := path.Join(crdsDir, crdPathSuffix)

	exist, _ := util.PathExists(crdsPath)
	if !exist {
		klog.V(2).InfoS("[runUnpack] CRD yaml files do not exist, unpacking tar file", "unpackDir", crdsDir)
		if err = validation.ValidateTarball(crdsTarPath, validation.ValidateCrdsTarBall); err != nil {
			return fmt.Errorf("[unpack] inValid crd tar, err: %w", err)
		}
		if err := util.Unpack(crdsTarPath, crdsDir); err != nil {
			return fmt.Errorf("[unpack] failed to unpack crd tar, err: %w", err)
		}
	} else {
		klog.V(2).InfoS("[unpack] These crds yaml files have been decompressed in the path", "path", crdsPath, "karmada", klog.KObj(data))
	}

	klog.V(2).InfoS("[unpack] Successfully unpacked crd tar", "karmada", klog.KObj(data))
	return nil
}

func postCheck(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("post-check task invoked with an invalid data struct")
	}

	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return fmt.Errorf("[post-check] failed to get CRD dir, err: %w", err)
	}

	for _, archive := range validation.CrdsArchive {
		expectedDir := filepath.Join(crdsDir, archive)
		exist, _ := util.PathExists(expectedDir)
		if !exist {
			return fmt.Errorf("[post-check] Lacking the necessary file path: %s", expectedDir)
		}
	}

	klog.V(2).InfoS("[post-check] Successfully post-check the crd tar archive", "karmada", klog.KObj(data))
	return nil
}

func existCrdsTar(crdsDir string) bool {
	files := util.ListFiles(crdsDir)
	klog.V(2).InfoS("[existCrdsTar] Checking for CRD tar file in directory", "directory", crdsDir)

	for _, file := range files {
		if strings.Contains(file.Name(), crdsFileSuffix) && file.Size() > 0 {
			return true
		}
	}
	return false
}

func getCrdsDir(data InitData) (string, error) {
	crdTarball := data.CrdTarball()
	key := strings.TrimSpace(crdTarball.HTTPSource.URL)
	hash := sha256.Sum256([]byte(key))
	hashedKey := hex.EncodeToString(hash[:])
	return path.Join(data.DataDir(), "cache", hashedKey), nil
}
