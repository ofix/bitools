package core

import (
	"net/http"
	"tls"
)

func ServeHTTP2(listen string, cert tls.Certificate) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		rw := &streamWrapper{
			r: r.Body,
			w: w,
		}
		defer r.Body.Close()
		for {
			f, err := DecodeFrame(rw)
			if err != nil {
				return
			}
			_ = EncodeFrame(rw, f)
			fl.Flush()
		}
	})
	srv := &http.Server{
		Addr:    listen,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	return srv.ListenAndServeTLS("", "")
}

type streamWrapper struct {
	r io.Reader
	w io.Writer
}
func (sw *streamWrapper) Read(b []byte) (int, error)  { return sw.r.Read(b) }
func (sw *streamWrapper) Write(b []byte) (int, error) { return sw.w.Write(b) }
