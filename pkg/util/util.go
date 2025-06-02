package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

func FormatStruct(obj any) string {
	json, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(json)
}

func GetEnv(key, value string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return value
}

func GenerateTemplate(tmpl, name string, config any) (string, error) {
	// 创建一个新的模板
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	// 创建一个缓冲区用于存储渲染后的结果
	var buf bytes.Buffer

	// 执行模板，将变量值填充到模板中
	err = t.Execute(&buf, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	// 返回渲染后的脚本内容
	return buf.String(), nil
}
