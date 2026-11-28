package forward

import (
	"proxy/core"
)

func Serve(proto string, mux *core.StreamMux) (int, error) {
	switch proto {
	case "socks5":
		return ServeSocks5(mux)
	case "connect":
		return ServeConnect(mux)
	default:
		return 0, fmt.Errorf("unsupported proto")
	}
}
