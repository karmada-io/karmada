/*
Copyright 2022 The Karmada Authors.

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

package service

//go:generate protoc --gogo_opt=paths=source_relative -I . -I ../../.. -I ../../../vendor --gogo_out=plugins=grpc:. --gogo_opt=Mpkg/estimator/pb/generated.proto=github.com/karmada-io/karmada/pkg/estimator/pb service.proto
//go:generate mockery --config=mockery.yaml
