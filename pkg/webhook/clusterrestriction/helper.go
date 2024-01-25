/*
Copyright 2021 The Karmada Authors.

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

package clusterrestriction

import (
	"strings"

	"golang.org/x/exp/slices"
	authenticationv1 "k8s.io/api/authentication/v1"

	"github.com/karmada-io/karmada/pkg/karmadactl/register"
)

// AgentIdentity determines karmada-agent's name, which is equal to Cluster name,
// from the given UserInfo.
// This method returns:
// - agentName: the username of the karmada-agent, and
// - isAgent: the UserInfo represents an karmada-agent or not
func AgentIdentity(userInfo authenticationv1.UserInfo) (agentName string, isAgent bool) {
	isAgent = strings.HasPrefix(userInfo.Username, register.ClusterPermissionPrefix) && slices.Contains(userInfo.Groups, register.ClusterPermissionGroups)
	if isAgent {
		agentName = userInfo.Username
	}
	return
}
