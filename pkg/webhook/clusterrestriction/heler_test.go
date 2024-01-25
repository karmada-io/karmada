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
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
)

func TestAgentIdentity(t *testing.T) {
	testCases := []struct {
		name              string
		userInfo          authenticationv1.UserInfo
		expectedAgentName string
		expectedIsAgent   bool
	}{
		{
			name: `empty should return ("", false)`,
		},
		{
			name: `both username and groups is karmada-agent should return (<karmada-agent name>, true)`,
			userInfo: authenticationv1.UserInfo{
				Username: "system:node:cluster-1",
				Groups:   []string{"system:nodes", "extra-group"},
			},
			expectedAgentName: "system:node:cluster-1",
			expectedIsAgent:   true,
		},
		{
			name: `username is karmada-agent but groups is not karmada-agent should return ("", false)`,
			userInfo: authenticationv1.UserInfo{
				Username: "system:node:cluster-1",
				Groups:   []string{"not-karmada-agent"},
			},
		},
		{
			name: `username is not karmada-agent but groups is karmada-agent should return ("", false)`,
			userInfo: authenticationv1.UserInfo{
				Username: "not-karmada-agent",
				Groups:   []string{"system:nodes"},
			},
		},
		{
			name: `both username and groups is not karmada-agent should return ("", false)`,
			userInfo: authenticationv1.UserInfo{
				Username: "not-karmada-agent",
				Groups:   []string{"not-karmada-agent"},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			gotAgentName, gotIsAgent := AgentIdentity(testCase.userInfo)
			if gotAgentName != testCase.expectedAgentName {
				t.Errorf("agentName: want %v, but got %v", testCase.expectedAgentName, gotAgentName)
			}
			if gotIsAgent != testCase.expectedIsAgent {
				t.Errorf("isAgent: want %v, but got %v", testCase.expectedIsAgent, gotIsAgent)
			}
		})
	}
}
