package profileflag

import (
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// Options are options for pprof.
type Options struct {
	// EnableProfile is the flag about whether to enable pprof profiling.
	EnableProfile bool
	// ProfilePort is the TCP address for pprof profiling.
	// Defaults to 127.0.0.1:6060 if unspecified.
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
			if err := http.ListenAndServe(opts.ProfilingBindAddress, mux); err != nil {
				klog.Errorf("Failed to enable profiling: %v", err)
				os.Exit(1)
			}
		}()
	}
}
