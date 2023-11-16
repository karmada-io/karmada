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

package store

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage"
)

func convertToMetaV1GetOptions(o storage.GetOptions) metav1.GetOptions {
	return metav1.GetOptions{
		ResourceVersion: o.ResourceVersion,
	}
}

func convertToMetaV1ListOptions(o storage.ListOptions) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector:        o.Predicate.Label.String(),
		FieldSelector:        o.Predicate.Field.String(),
		AllowWatchBookmarks:  o.Predicate.AllowWatchBookmarks,
		ResourceVersion:      o.ResourceVersion,
		ResourceVersionMatch: o.ResourceVersionMatch,
		Limit:                o.Predicate.Limit,
		Continue:             o.Predicate.Continue,
	}
}
