package serial

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"marketplace-yaga/pkg/messages"
	"runtime"
	"sync"
	"testing"
)

type testPort struct {
	write func([]byte) (int, error)
}

func (p testPort) Write(bs []byte) (int, error) { return p.write(bs) }
func (testPort) Close() error                   { return nil }

func useTestPort(t *testing.T, p io.WriteCloser) {
	t.Helper()
	previous := port
	port = p
	t.Cleanup(func() { port = previous })
}

func TestWriteJSONCompletesShortWrites(t *testing.T) {
	var output bytes.Buffer
	useTestPort(t, testPort{write: func(bs []byte) (int, error) {
		if len(bs) > 3 {
			bs = bs[:3]
		}
		return output.Write(bs)
	}})
	message := messages.NewEnvelope().WithType("heartbeat").Wrap(map[string]string{"Status": "ok"})

	if err := NewBlockingWriter().WriteJSON(message); err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	expected = append(expected, '\n')
	if !bytes.Equal(output.Bytes(), expected) {
		t.Fatalf("got truncated message %q, want %q", output.Bytes(), expected)
	}
}

func TestWriteStopsOnNoProgress(t *testing.T) {
	useTestPort(t, testPort{write: func([]byte) (int, error) { return 0, nil }})
	n, err := NewBlockingWriter().Write([]byte("message\n"))
	if n != 0 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got (%d, %v), want (0, %v)", n, err, io.ErrShortWrite)
	}
	if err := NewBlockingWriter().WriteJSON(map[string]string{"Status": "ok"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v, want %v", err, io.ErrShortWrite)
	}
}

func TestWriteReturnsPartialCountAndError(t *testing.T) {
	wantErr := errors.New("serial write failed")
	calls := 0
	useTestPort(t, testPort{write: func(bs []byte) (int, error) {
		calls++
		if calls == 1 {
			return 2, nil
		}
		return 1, wantErr
	}})
	n, err := NewBlockingWriter().Write([]byte("message\n"))
	if n != 3 || !errors.Is(err, wantErr) || calls != 2 {
		t.Fatalf("got (%d, %v) after %d calls, want (3, %v) after 2 calls", n, err, calls, wantErr)
	}
}

func TestWriteJSONReturnsPortError(t *testing.T) {
	wantErr := errors.New("serial write failed")
	useTestPort(t, testPort{write: func([]byte) (int, error) { return 0, wantErr }})
	if err := NewBlockingWriter().WriteJSON(map[string]string{"Status": "ok"}); !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}

func TestWritersShareRecordLock(t *testing.T) {
	var output bytes.Buffer
	var outputLock sync.Mutex
	useTestPort(t, testPort{write: func(bs []byte) (int, error) {
		if len(bs) > 7 {
			bs = bs[:7]
		}
		outputLock.Lock()
		n, err := output.Write(bs)
		outputLock.Unlock()
		runtime.Gosched()
		return n, err
	}})

	const workers = 8
	const recordsPerWorker = 16
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			writer := NewBlockingWriter()
			for record := 0; record < recordsPerWorker; record++ {
				message := (&messages.Envelope{
					Timestamp: 1,
					Type:      "heartbeat",
					ID:        fmt.Sprintf("%d-%d", worker, record),
				}).Wrap(map[string]string{"Status": "ok"})
				var err error
				if worker%2 == 0 {
					err = writer.WriteJSON(message)
				} else {
					var bs []byte
					bs, err = json.Marshal(message)
					if err == nil {
						_, err = writer.Write(append(bs, '\n'))
					}
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if !bytes.HasSuffix(output.Bytes(), []byte{'\n'}) {
		t.Fatal("last record has no newline")
	}
	lines := bytes.Split(bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), []byte{'\n'})
	if len(lines) != workers*recordsPerWorker {
		t.Fatalf("got %d records, want %d", len(lines), workers*recordsPerWorker)
	}
	ids := make(map[string]bool)
	for _, line := range lines {
		envelope, err := messages.UnmarshalEnvelope(line)
		if err != nil {
			t.Fatalf("invalid record %q: %v", line, err)
		}
		if ids[envelope.ID] {
			t.Fatalf("duplicate record %s", envelope.ID)
		}
		var payload struct{ Status string }
		if err := messages.UnmarshalPayload(line, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != "ok" {
			t.Fatalf("corrupted payload in record %s: %+v", envelope.ID, payload)
		}
		ids[envelope.ID] = true
	}
}

func TestUninitializedWriter(t *testing.T) {
	useTestPort(t, nil)
	writer := NewBlockingWriter()
	if _, err := writer.Write([]byte("message")); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	if err := writer.WriteJSON(map[string]string{"Status": "ok"}); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	if err := writer.Close(); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
}
