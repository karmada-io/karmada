package tasks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
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
		},
	}
}

func runPrepareCrds(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("prepare-crds task invoked with an invalid data struct")
	}

	crdsDir := path.Join(data.DataDir(), data.KarmadaVersion())
	klog.V(4).InfoS("[prepare-crds] Running prepare-crds task", "karmada", klog.KObj(data))
	klog.V(2).InfoS("[prepare-crds] Using crd folder", "folder", crdsDir, "karmada", klog.KObj(data))

	return nil
}

func skipCrdsDownload(r workflow.RunData) (bool, error) {
	data, ok := r.(InitData)
	if !ok {
		return false, errors.New("prepare-crds task invoked with an invalid data struct")
	}

	crdsDir := path.Join(data.DataDir(), data.KarmadaVersion())
	if exist, err := util.PathExists(crdsDir); !exist || err != nil {
		return false, err
	}

	if !existCrdsTar(crdsDir) {
		return false, nil
	}

	klog.V(2).InfoS("[download-crds] Skip download crd yaml files, the crd tar exists on disk", "karmada", klog.KObj(data))
	return true, nil
}

func runCrdsDownload(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("download-crds task invoked with an invalid data struct")
	}

	var (
		crdsDir     = path.Join(data.DataDir(), data.KarmadaVersion())
		crdsTarPath = path.Join(crdsDir, crdsFileSuffix)
	)

	exist, err := util.PathExists(crdsDir)
	if err != nil {
		return err
	}
	if !exist {
		if err := os.MkdirAll(crdsDir, 0700); err != nil {
			return err
		}
	}

	if !existCrdsTar(crdsDir) {
		err := util.DownloadFile(data.CrdsRemoteURL(), crdsTarPath)
		if err != nil {
			return fmt.Errorf("failed to download crd tar, err: %w", err)
		}
	} else {
		klog.V(2).InfoS("[download-crds] The crd tar exists on disk", "path", crdsDir, "karmada", klog.KObj(data))
	}

	klog.V(2).InfoS("[download-crds] Successfully downloaded crd package for remote url", "karmada", klog.KObj(data))
	return nil
}

func runUnpack(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("unpack task invoked with an invalid data struct")
	}

	var (
		crdsDir     = path.Join(data.DataDir(), data.KarmadaVersion())
		crdsTarPath = path.Join(crdsDir, crdsFileSuffix)
		crdsPath    = path.Join(crdsDir, crdPathSuffix)
	)

	// TODO: check whether crd yaml is valid.
	exist, _ := util.PathExists(crdsPath)
	if !exist {
		if err := util.Unpack(crdsTarPath, crdsDir); err != nil {
			return fmt.Errorf("[unpack] failed to unpack crd tar, err: %w", err)
		}
	} else {
		klog.V(2).InfoS("[unpack] These crds yaml files have been decompressed in the path", "path", crdsPath, "karmada", klog.KObj(data))
	}

	klog.V(2).InfoS("[unpack] Successfully unpacked crd tar", "karmada", klog.KObj(data))
	return nil
}

func existCrdsTar(crdsDir string) bool {
	files := util.ListFiles(crdsDir)

	for _, file := range files {
		if strings.Contains(file.Name(), crdsFileSuffix) && file.Size() > 0 {
			return true
		}
	}
	return false
}
