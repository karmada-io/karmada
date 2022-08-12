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
