package explorer

// DecoderInjector is used by the ControllerManager to inject decoder into webhook handlers.
type DecoderInjector interface {
	InjectDecoder(*Decoder)
}

// InjectDecoderInto will set decoder on i and return the result if it implements Decoder.
// Returns false if i does not implement Decoder.
func InjectDecoderInto(decoder *Decoder, i interface{}) bool {
	if s, ok := i.(DecoderInjector); ok {
		s.InjectDecoder(decoder)
		return true
	}
	return false
}
