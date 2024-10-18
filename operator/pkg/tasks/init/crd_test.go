/*
Copyright 2024 The Karmada Authors.

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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestRunPrepareCrds(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunPrepareCrds_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "prepare-crds task invoked with an invalid data struct",
		},
		{
			name: "RunPrepareCrds_ValidCrdsDirectory_CrdsDirectoryIsValid",
			runData: &TestInitData{
				Name:          name,
				Namespace:     namespace,
				DataDirectory: constants.KarmadaDataDir,
				CrdTarballArchive: operatorv1alpha1.CRDTarball{
					HTTPSource: &operatorv1alpha1.HTTPSource{
						URL: "https://www.example.com/crd-tarball",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runPrepareCrds(test.runData)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestRunSkipCrdsDownload(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name     string
		runData  workflow.RunData
		prep     func(workflow.RunData) error
		cleanup  func(workflow.RunData) error
		wantSkip bool
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "RunSkipCrdsDownload_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData:  &MyTestData{},
			prep:     func(workflow.RunData) error { return nil },
			cleanup:  func(workflow.RunData) error { return nil },
			wantSkip: false,
			wantErr:  true,
			errMsg:   "prepare-crds task invoked with an invalid data struct",
		},
		{
			name: "RunSkipCrdsDownload_WithCrdsTar_CrdsDownloadSkipped",
			runData: &TestInitData{
				Name:          name,
				Namespace:     namespace,
				DataDirectory: filepath.Join(os.TempDir(), "crds-test"),
				CrdTarballArchive: operatorv1alpha1.CRDTarball{
					HTTPSource: &operatorv1alpha1.HTTPSource{
						URL: "https://www.example.com/crd-tarball",
					},
				},
			},
			prep: runSkipCrdsDownloadPrep,
			cleanup: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if err := os.RemoveAll(data.DataDirectory); err != nil {
					return fmt.Errorf("failed to cleanup data directory %s, got error: %v", data.DataDirectory, err)
				}
				return nil
			},
			wantSkip: true,
			wantErr:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Fatalf("failed to prep before skipping downloading crds, got error: %v", err)
			}
			skipDownload, err := skipCrdsDownload(test.runData)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if skipDownload != test.wantSkip {
				t.Errorf("expected crds download status to be %t, but got %t", test.wantSkip, skipDownload)
			}
			if err := test.cleanup(test.runData); err != nil {
				t.Errorf("failed to cleanup test environment artifacts, got error: %v", err)
			}
		})
	}
}

func TestRunCrdsDownload(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		cleanup func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCrdsDownload_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			cleanup: func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "download-crds task invoked with an invalid data struct",
		},
		{
			name: "RunCrdsDownload_DownloadingCRDs_CRDsDownloadedSuccessfully",
			runData: &TestInitData{
				Name:          name,
				Namespace:     namespace,
				DataDirectory: filepath.Join(os.TempDir(), "crds-test"),
				CrdTarballArchive: operatorv1alpha1.CRDTarball{
					HTTPSource: &operatorv1alpha1.HTTPSource{
						URL: "https://github.com/karmada-io/karmada/releases/download/v1.11.1/crds.tar.gz",
					},
				},
			},
			cleanup: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if err := os.RemoveAll(data.DataDirectory); err != nil {
					return fmt.Errorf("failed to cleanup data directory %s, got error: %v", data.DataDirectory, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runCrdsDownload(test.runData)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.cleanup(test.runData); err != nil {
				t.Errorf("failed to cleanup test environment artifacts, got error: %v", err)
			}
		})
	}
}

func TestRunUnpack(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		cleanup func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunUnpack_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			cleanup: func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "unpack task invoked with an invalid data struct",
		},
		{
			name: "RunUnpack_CRDYamlFilesNotExist_CRDTarUnpacked",
			runData: &TestInitData{
				Name:          name,
				Namespace:     namespace,
				DataDirectory: filepath.Join(os.TempDir(), "crds-test"),
				CrdTarballArchive: operatorv1alpha1.CRDTarball{
					HTTPSource: &operatorv1alpha1.HTTPSource{
						URL: "https://github.com/karmada-io/karmada/releases/download/v1.11.1/crds.tar.gz",
					},
				},
			},
			prep: func(rd workflow.RunData) error {
				if err := runCrdsDownload(rd); err != nil {
					return fmt.Errorf("failed to download crds, got error: %v", err)
				}
				return nil
			},
			cleanup: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if err := os.RemoveAll(data.DataDirectory); err != nil {
					return fmt.Errorf("failed to cleanup data directory %s, got error: %v", data.DataDirectory, err)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Fatalf("failed to prep before unpacking crds, got error: %v", err)
			}
			err := runUnpack(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.cleanup(test.runData); err != nil {
				t.Errorf("failed to cleanup test environment artifacts, got error: %v", err)
			}
		})
	}
}

// runSkipCrdsDownloadPrep prepares the CRD directory and creates a dummy CRD tar file.
// It is used as part of a test workflow to simulate the presence of CRD data without
// actually downloading or validating real CRD content.
func runSkipCrdsDownloadPrep(rd workflow.RunData) error {
	data := rd.(*TestInitData)
	crdsDir, err := getCrdsDir(data)
	if err != nil {
		return fmt.Errorf("failed to get crds directory, got error: %v", err)
	}

	// Create CRD test directories recursively.
	if err := os.MkdirAll(crdsDir, 0700); err != nil {
		return fmt.Errorf("failed to create crds directory recursively, got error: %v", err)
	}

	// Create CRD test file.
	crdsTarFile := filepath.Join(crdsDir, crdsFileSuffix)
	file, err := os.Create(crdsTarFile)
	if err != nil {
		return fmt.Errorf("failed to create crds tar file %s, got error: %v", crdsTarFile, err)
	}
	defer file.Close()

	// Write dummy data to the tar file - not necessary valid.
	if _, err := file.WriteString("test data"); err != nil {
		return fmt.Errorf("failed to write to tar file, got error: %v", err)
	}

	return nil
}
