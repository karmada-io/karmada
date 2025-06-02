/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/20 11:14:27
 Desc     :
*/

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func HttpGet(url string) (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func HttpPost(url string, params any) (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	postData := []byte(data)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http post failed, url: %s, status code: %d, body: %s", url, resp.StatusCode, string(body))
	}
	return string(body), nil
}

func HttpPut(url string, params any) (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	postData := []byte(data)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(postData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	return string(body), nil
}
