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
	"net"
	"os"
	"reflect"
	"slices"
	"testing"
)

func stringInslice(target string, strArray []string) bool {
	return slices.Contains(strArray, target)
}

func TestStringToNetIP(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want net.IP
	}{
		{
			name: "get ipv4",
			addr: "10.1.1.1",
			want: net.ParseIP("10.1.1.1"),
		},
		{
			name: "get ipv6",
			addr: "fe80::f4e6:6514:7461:9698",
			want: net.ParseIP("fe80::f4e6:6514:7461:9698"),
		},
		{
			name: "get invalid ip",
			addr: "120.1",
			want: net.ParseIP("127.0.0.1"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringToNetIP(tt.addr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StringToNetIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlagsIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want []net.IP
	}{
		{
			name: "all ips are valid",
			ip:   "10.0.0.1,10.0.0.2",
			want: []net.IP{
				net.ParseIP("10.0.0.1"),
				net.ParseIP("10.0.0.2"),
			},
		},
		{
			name: "have invalid ip",
			ip:   "10.0.0,10.0.0.2",
			want: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP("10.0.0.2"),
			},
		},
		{
			name: "have ipv6 and ipv4",
			ip:   "fe80::f4e6:6514:7461:9698, 10.0.0.1",
			want: []net.IP{
				net.ParseIP("fe80::f4e6:6514:7461:9698"),
				net.ParseIP("127.0.0.1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlagsIP(tt.ip); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlagsIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlagsDNS(t *testing.T) {
	tests := []struct {
		name string
		dns  string
		want []string
	}{
		{
			name: "get dns from flags",
			dns:  "github.com,karmada.io,kubernetes.io",
			want: []string{
				"github.com", "karmada.io", "kubernetes.io",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlagsDNS(tt.dns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlagsDNS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInternetIP(t *testing.T) {
	got, err := InternetIP()
	if got == nil {
		t.Errorf("InternetIP() want return not nil, but return nil")
	}
	if err != nil {
		t.Errorf("InternetIP() want return not error, but return error")
	}
}

func TestFileToBytes(t *testing.T) {
	type args struct {
		path string
		name string
	}
	tests := []struct {
		name        string
		createtmp   bool
		tempcontent string
		args        args
		want        []byte
		wantErr     bool
	}{
		{
			name:        "the file is not exist",
			createtmp:   false,
			tempcontent: "",
			args: args{
				path: "a-not-exits-path",
				name: "a-not-exit-file.txt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:        "the file is not exist",
			createtmp:   true,
			tempcontent: "hello, world",
			args: args{
				path: "a-not-exits-path-" + randString(),
				name: "a-not-exit-file-" + randString() + ".txt",
			},
			want:    []byte("hello, world"),
			wantErr: false,
		},
		{
			// Regression test: FileToBytes must read the entire file rather than
			// relying on a single Read call to fill the buffer, since io.Reader
			// does not guarantee that. A multi-megabyte file exercises this path.
			name:      "large content is read in full, not truncated",
			createtmp: true,
			args: args{
				path: "a-not-exits-path-" + randString(),
				name: "a-not-exit-file-" + randString() + ".txt",
			},
			wantErr: false,
		},
	}
	for i := range tests {
		tt := &tests[i]
		if tt.name == "large content is read in full, not truncated" {
			content := make([]byte, 5*1024*1024)
			for j := range content {
				content[j] = byte(j % 251)
			}
			tt.tempcontent = string(content)
			tt.want = content
		}
		if tt.createtmp {
			err := os.Mkdir(tt.args.path, 0755)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(tt.args.path + "/" + tt.args.name)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Write([]byte(tt.tempcontent))
			if err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileToBytes(tt.args.path, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				if len(got) != len(tt.want) {
					t.Errorf("FileToBytes() length = %d, want length %d", len(got), len(tt.want))
					return
				}
				for j := range got {
					if got[j] != tt.want[j] {
						t.Errorf("FileToBytes() content mismatch at byte %d: got %d, want %d (lengths match at %d, so this is not a truncation)", j, got[j], tt.want[j], len(got))
						return
					}
				}
			}
		})
		if tt.createtmp {
			os.RemoveAll(tt.args.path)
		}
	}
}

func TestBytesToFile(t *testing.T) {
	type args struct {
		path string
		name string
		data []byte
	}
	tests := []struct {
		name      string
		createtmp bool
		args      args
		wantErr   bool
	}{
		{
			name:      "a not exist file",
			createtmp: false,
			args: args{
				path: "temp-kubeconfig-" + randString(),
				name: randString() + ".kubeconfig",
				data: []byte("hello world"),
			},
			wantErr: true,
		},
		{
			name:      "a not exist file",
			createtmp: true,
			args: args{
				path: "temp-kubeconfig-" + randString(),
				name: randString() + ".kubeconfig",
				data: []byte("hello world"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		if tt.createtmp {
			err := os.Mkdir(tt.args.path, 0755)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(tt.args.path + "/" + tt.args.name)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			if err := BytesToFile(tt.args.path, tt.args.name, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("BytesToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
		if tt.createtmp {
			os.RemoveAll(tt.args.path)
		}
	}
}

func TestMapToString(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name: "map is not null",
			labels: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			want: []string{"foo1=bar1,foo2=bar2", "foo2=bar2,foo1=bar1"},
		},
		{
			name:   "map is null",
			labels: map[string]string{},
			want:   []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapToString(tt.labels); !stringInslice(got, tt.want) {
				t.Errorf("MapToString() = %v, want string in list%v", got, tt.want)
			}
		})
	}
}

func TestStringToMap(t *testing.T) {
	tests := []struct {
		name   string
		labels string
		want   map[string]string
	}{
		{
			name:   "valid map string",
			labels: "foo=bar",
			want:   map[string]string{"foo": "bar"},
		},
		{
			name:   "invalid map string",
			labels: "foo=bar,bar=foo",
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringToMap(tt.labels); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StringToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
