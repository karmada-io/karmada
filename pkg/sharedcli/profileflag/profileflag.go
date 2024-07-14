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

package profileflag

import (
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

const (
	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	// References:
	// - https://en.wikipedia.org/wiki/Slowloris_(computer_security)
	ReadHeaderTimeout = 32 * time.Second
	// WriteTimeout is the amount of time allowed to write the
	// request data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	WriteTimeout = 5 * time.Minute
	// ReadTimeout is the amount of time allowed to read
	// response data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	ReadTimeout = 5 * time.Minute
)

// Options are options for pprof.
type Options struct {
	// EnableProfile is the flag about whether to enable pprof profiling.
	EnableProfile bool
	// ProfilingBindAddress is the TCP address for pprof profiling.
	// Defaults to :6060 if unspecified.
	ProfilingBindAddress string
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.EnableProfile, "enable-pprof", false, "Enable profiling via web interface host:port/debug/pprof/.")
	fs.StringVar(&o.ProfilingBindAddress, "profiling-bind-address", ":6060", "The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled.")
}

func installHandlerForPProf(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// ListenAndServe start a http server to enable pprof.
func ListenAndServe(opts Options) {
	if opts.EnableProfile {
		mux := http.NewServeMux()
		installHandlerForPProf(mux)
		klog.Infof("Starting profiling on port %s", opts.ProfilingBindAddress)
		go func() {
			httpServer := http.Server{
				Addr:              opts.ProfilingBindAddress,
				Handler:           mux,
				ReadHeaderTimeout: ReadHeaderTimeout,
				WriteTimeout:      WriteTimeout,
				ReadTimeout:       ReadTimeout,
			}
			if err := httpServer.ListenAndServe(); err != nil {
				klog.Errorf("Failed to enable profiling: %v", err)
				os.Exit(1)
			}
		}()
	}
}
