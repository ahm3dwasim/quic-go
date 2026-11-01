package http3

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/testutils/events"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestQlogNoFrameCreatedOnRequestHeaderWriteError(t *testing.T) {
	var eventRecorder events.Recorder
	req := httptest.NewRequest(http.MethodGet, "https://quic-go.net/", nil)
	require.Error(t, newRequestWriter().WriteRequestHeader(failingWriter{}, req, false, 42, &eventRecorder))
	require.Empty(t, eventRecorder.Events())
}

func TestQlogNoFrameCreatedOnTrailerWriteError(t *testing.T) {
	var eventRecorder events.Recorder
	_, err := writeTrailers(failingWriter{}, http.Header{"Foo": {"bar"}}, 42, &eventRecorder)
	require.Error(t, err)
	require.Empty(t, eventRecorder.Events())
}

func newFailingStream(t *testing.T) (*Stream, *events.Recorder) {
	var eventRecorder events.Recorder
	mockCtrl := gomock.NewController(t)
	qstr := NewMockDatagramStream(mockCtrl)
	qstr.EXPECT().StreamID().Return(quic.StreamID(42)).AnyTimes()
	qstr.EXPECT().Write(gomock.Any()).Return(0, errors.New("write failed")).AnyTimes()
	clientConn, _ := newConnPair(t)
	conn := newRawConn(clientConn, false, nil, nopControlStrHandler, &eventRecorder, nil)
	return newStream(qstr, conn, nil, func(io.Reader, *headersFrame) error { return nil }, &eventRecorder), &eventRecorder
}

func TestQlogNoFrameCreatedOnDataWriteError(t *testing.T) {
	str, eventRecorder := newFailingStream(t)
	_, err := str.Write([]byte("foobar"))
	require.Error(t, err)
	require.Empty(t, eventRecorder.Events())
}

func TestQlogNoFrameCreatedOnResponseHeaderWriteError(t *testing.T) {
	str, eventRecorder := newFailingStream(t)
	rw := newResponseWriter(str, nil, false, slog.Default())
	rw.WriteHeader(http.StatusOK)
	require.Error(t, rw.FlushError())
	require.Empty(t, eventRecorder.Events())
}
