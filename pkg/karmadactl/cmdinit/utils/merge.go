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

package utils

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

// KarmadaComponentCommand 合并默认参数和用户给的参数
func KarmadaComponentCommand(defaultArgs, extraArgs []string) []string {
	// 没有参数直接返回
	if len(extraArgs) == 0 {
		return defaultArgs
	}
	// 校验参数
	args, err := validateArgs(extraArgs)
	if err != nil {
		klog.Errorf("%v", err)
		return nil
	}

	// 合并参数
	return mergeCommandArgs(defaultArgs, args)
}

// 正则校验用户给的参数
// format: --key=value
func validateArgs(args []string) ([]string, error) {
	argPattern := regexp.MustCompile(`^--[a-zA-Z0-9\-]+=(.*)?$`)
	for _, arg := range args {
		if !argPattern.MatchString(arg) {
			return nil, fmt.Errorf("invalid argument: %s", arg)
		}
	}
	return args, nil
}

func mergeCommandArgs(defaultArgs, extraArgs []string) []string {
	finalArgs := make([]string, 0, len(defaultArgs))
	argMap := make(map[string]string)

	// 将默认参数转成map
	for _, arg := range defaultArgs[1:] { // 跳过第一个参数
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2) // 避免value中含有=
			if len(parts) == 2 {
				argMap[parts[0]] = parts[1]
			} else {
				argMap[parts[0]] = ""
			}
		}
	}

	// extra 参数覆盖默认参数
	for _, arg := range extraArgs {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2) // 避免value中含有=
			if len(parts) == 2 {
				argMap[parts[0]] = parts[1]
			} else {
				argMap[parts[0]] = ""
			}
		}
	}

	// 构造命令行参数
	finalArgs = append(finalArgs, defaultArgs[0]) // 加上命令名
	for k, v := range argMap {
		finalArgs = append(finalArgs, fmt.Sprintf("%s=%s", k, v))
	}

	return finalArgs
}
