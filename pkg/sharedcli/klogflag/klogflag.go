package klogflag

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// Add used to add klog flags to specified flag set.
func Add(fs *pflag.FlagSet) {
	// Since klog only accepts golang flag set, so introduce a shim here.
	flagSetShim := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(flagSetShim)

	fs.AddGoFlagSet(flagSetShim)
}
