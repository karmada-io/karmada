package hash

import (
	"hash"

	"github.com/kr/pretty"
)

// DeepHashObject writes specified object to hash using the pretty library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	pretty.Fprintf(hasher, "%# v", objectToWrite)
}
