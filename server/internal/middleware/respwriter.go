package middleware

import (
	"io"
	"net/http"
)

// statusRecorder wraps an http.ResponseWriter to remember the status code for
// the access logger (gin exposed this via c.Writer.Status()). It forwards Flush
// and ReadFrom so http.ServeContent (used by the SPA handler) keeps its
// streaming and Range behavior intact.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.written {
		s.status = code
		s.written = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.written {
		s.status = http.StatusOK
		s.written = true
	}
	return s.ResponseWriter.Write(b)
}

// Status returns the recorded status, defaulting to 200 when the handler wrote
// a body without an explicit WriteHeader, and to 200 when nothing was written.
func (s *statusRecorder) Status() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the wrapped ResponseWriter so http.NewResponseController can
// reach the underlying connection for SetWriteDeadline (needed by the SSE
// stream handler to clear the server's WriteTimeout) and other optional
// interfaces it doesn't already forward.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s *statusRecorder) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		if !s.written {
			s.status = http.StatusOK
			s.written = true
		}
		return rf.ReadFrom(r)
	}
	return io.Copy(s.ResponseWriter, r)
}
