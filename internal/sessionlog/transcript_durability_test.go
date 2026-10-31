package sessionlog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/atomicfile"
)

// This is the actual old writer's interruption window: write only a prefix of
// the next append over the closing tail, leaving any old tail bytes in place.
func interruptLegacyTranscriptAppend(t *testing.T, path string, committed, pending []byte, cut int) []byte {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(pending[:cut], int64(len(committed)-len(transcriptTail))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTranscriptRecovery_InterruptedLegacyAppendReopensAndSearches(t *testing.T) {
	pendingBody, err := marshalEvent(TranscriptEvent{Seq: 3, Time: time.Unix(1, 0).UTC(), Kind: EventToolResult,
		Text: Lines("unfinished event with nested-looking text: ] } { \\\" and unicode λ")})
	if err != nil {
		t.Fatal(err)
	}
	event := ",\n" + indentEvent(pendingBody)
	pending := []byte(event + transcriptTail)
	unicodeCut := bytes.Index(pending, []byte("λ")) + 1
	for _, cut := range []int{1, 2, 4, 6, 7, 12, len(event) / 2, unicodeCut, len(event) - 1, len(event), len(pending) - 2} {
		t.Run(strconv.Itoa(cut), func(t *testing.T) {
			s, _ := installStore(t, "session-1", false)
			ctx := WithTurn(WithAgent(context.Background(), "coder", "coder-1"), 4)
			tr := TranscriptFrom(ctx)
			tr.Input(ctx, "stable_needle_before: first completed event")
			tr.ToolResult(ctx, "read_file", "call-1", "stable_needle_before: second completed event", false)
			path := sessionPath(s, "session-1", "coder-1", TranscriptFile)
			committed := []byte(readFile(t, path))
			// Preserve extensions and exact large integer values during recovery
			// and subsequent snapshot publication, not only known struct fields.
			committed = bytes.Replace(committed, []byte(`"events": [`), []byte("\"extension\": {\"large\": 9007199254740993},\n  \"events\": ["), 1)
			if err := os.WriteFile(path, committed, 0o600); err != nil {
				t.Fatal(err)
			}
			torn := interruptLegacyTranscriptAppend(t, path, committed, pending, cut)
			hits, err := Search(s.workspace, "stable_needle_before", SearchOptions{Session: "session-1", Agent: "coder"})
			if err != nil || len(hits) != 2 {
				t.Errorf("search lost completed history before reopen: hits=%+v error=%v", hits, err)
			}
			if got := []byte(readFile(t, path)); !bytes.Equal(got, torn) {
				t.Fatal("search modified the interrupted transcript")
			}
			reopened := NewStore(Options{Workspace: s.workspace, Session: "session-1"})
			Install(reopened)
			recorder := TranscriptFrom(ctx)
			recorder.Input(ctx, "resumed_recording")
			if n, err := reopened.Failures(); n != 0 {
				t.Errorf("recoverable append disabled recording: failures=%d error=%v broken=%v", n, err, recorder.broken)
			}
			data := []byte(readFile(t, path))
			var doc transcriptDoc
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Errorf("reopened transcript remains unreadable: %v", err)
				return
			}
			previous := 2
			if cut >= len(event) {
				previous++ // A whole decoded event is recoverable without its tail.
			}
			if len(doc.Events) != previous+1 || doc.Events[previous].Seq != previous+1 || Join(doc.Events[previous].Text) != "resumed_recording" {
				t.Fatalf("reopened sequence/history = %+v", doc.Events)
			}
			if Join(doc.Events[0].Text) != "stable_needle_before: first completed event" || Join(doc.Events[1].Text) != "stable_needle_before: second completed event" ||
				doc.Events[1].Tool != "read_file" || doc.Events[1].CallID != "call-1" || !bytes.Contains(data, []byte("9007199254740993")) {
				t.Fatalf("completed history or header extension changed: %s", data)
			}
			hits, err = Search(s.workspace, "resumed_recording", SearchOptions{Agent: "coder"})
			if err != nil || len(hits) != 1 || hits[0].Turn != 4 {
				t.Errorf("search did not find resumed recording: %+v, %v", hits, err)
			}
		})
	}
}

func TestTranscriptRecovery_AppendKeepsPreviousSnapshotForOpenReader(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, "committed before replacement")
	path := sessionPath(s, "session-1", "coder-1", TranscriptFile)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, path)
	reader, err := atomicfile.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	oldInfo, err := reader.Stat()
	if err != nil {
		t.Fatal(err)
	}
	tr.ToolResult(ctx, "read_file", "call-2", strings.Repeat("larger replacement\n", 500), false)
	old, err := io.ReadAll(reader)
	if err != nil || string(old) != before {
		t.Errorf("append mutated the only committed snapshot: previous bytes=%d now=%d error=%v", len(before), len(old), err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && os.SameFile(oldInfo, info) {
		t.Error("append wrote through the original inode instead of atomic replacement")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("existing transcript mode changed: %o", info.Mode().Perm())
	}
	var doc transcriptDoc
	readJSON(t, path, &doc)
	if len(doc.Events) != 2 || doc.Events[1].Seq != 2 {
		t.Fatalf("replacement lost event sequence: %+v", doc.Events)
	}
}

func TestTranscriptRecovery_UnfinishedTemporarySnapshotDoesNotHideHistory(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	TranscriptFrom(ctx).Input(ctx, "committed_before_interruption")
	path := sessionPath(s, "session-1", "coder-1", TranscriptFile)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".transcript.json.tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString(`{"events":[{"text":["uncommitted_snapshot`); err != nil {
		tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	Install(NewStore(Options{Workspace: s.workspace, Session: "session-1"}))
	TranscriptFrom(ctx).Final(ctx, "reopened_after_interruption", nil)
	for needle, want := range map[string]int{"committed_before_interruption": 1, "reopened_after_interruption": 1, "uncommitted_snapshot": 0} {
		hits, err := Search(s.workspace, needle, SearchOptions{Agent: "coder"})
		if err != nil || len(hits) != want {
			t.Errorf("search %q: hits=%+v error=%v, want %d", needle, hits, err, want)
		}
	}
}

func TestTranscriptRecovery_RejectsUnrecognizedCorruptionWithoutMutation(t *testing.T) {
	for _, corruption := range []string{"schema", "text_encoding", "session", "agent", "middle", "garbage", "duplicate_header"} {
		t.Run(corruption, func(t *testing.T) {
			s, _ := installStore(t, "session-1", false)
			ctx := WithAgent(context.Background(), "coder", "coder-1")
			TranscriptFrom(ctx).Input(ctx, "prior history")
			path := sessionPath(s, "session-1", "coder-1", TranscriptFile)
			data := readFile(t, path)
			data = strings.TrimSuffix(data, transcriptTail) + ",\n    {\"seq\": 2"
			switch corruption {
			case "schema":
				data = strings.Replace(data, TranscriptSchema, "unrecognized.transcript/9", 1)
			case "text_encoding":
				data = strings.Replace(data, `"text_encoding": `, `"unsupported_encoding": `, 1)
			case "session":
				data = strings.Replace(data, `"session": "session-1"`, `"session": "different-session"`, 1)
			case "agent":
				data = strings.Replace(data, `"agent": "coder-1"`, `"agent": "different-agent"`, 1)
			case "middle":
				data = strings.Replace(data, `"prior history"`, `"invalid\q escape"`, 1)
			case "garbage":
				data = strings.TrimSuffix(data, ",\n    {\"seq\": 2") + ",\n invalid content"
			case "duplicate_header":
				data = strings.Replace(data, `"events": [`, `"schema":"codenerd.transcript/1", "events": [`, 1)
			}
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			reopened := NewStore(Options{Workspace: s.workspace, Session: "session-1"})
			Install(reopened)
			TranscriptFrom(ctx).Input(ctx, "must not overwrite")
			if n, _ := reopened.Failures(); n != 1 {
				t.Errorf("unrecoverable corruption failures=%d, want 1", n)
			}
			if got := readFile(t, path); got != data {
				t.Fatal("recovery overwrote unrelated or malformed data")
			}
		})
	}
}

func TestTranscriptRecovery_FailedAppendCanRetryWithoutSequenceGap(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, "previous commit")
	path, previous := tr.path, readFile(t, tr.path)
	// Force a deterministic I/O error on this writer without changing process
	// permissions or injecting a global transport/file-operation replacement.
	tr.path = filepath.Join(t.TempDir(), "missing-parent", TranscriptFile)
	tr.Input(ctx, "failed append must not be recorded")
	tr.path = path
	if n, _ := s.Failures(); n != 1 || tr.broken || tr.seq != 1 {
		t.Fatalf("failed append state: failures=%d broken=%v sequence=%d", n, tr.broken, tr.seq)
	}
	if got := readFile(t, path); got != previous {
		t.Fatal("failed append changed previous committed bytes")
	}
	tr.Input(ctx, "successful retry")
	var doc transcriptDoc
	readJSON(t, path, &doc)
	if len(doc.Events) != 2 || doc.Events[1].Seq != 2 || Join(doc.Events[1].Text) != "successful retry" {
		t.Fatalf("retry did not continue committed sequence: %+v", doc.Events)
	}
}

func TestTranscriptRecovery_ConcurrentReadersSeeCompleteSnapshots(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, "initial_reader_needle")
	path := tr.path
	start, stop := make(chan struct{}), make(chan struct{})
	var readers, writers sync.WaitGroup
	for range 2 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for {
				f, err := atomicfile.Open(path)
				if err != nil {
					t.Error(err)
					return
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil || !json.Valid(data) {
					t.Errorf("reader saw an incomplete snapshot: error=%v bytes=%d", err, len(data))
					return
				}
				hits, err := Search(s.workspace, "initial_reader_needle", SearchOptions{Agent: "coder"})
				if err != nil || len(hits) != 1 {
					t.Errorf("concurrent search lost committed history: %+v, %v", hits, err)
					return
				}
				select {
				case <-stop:
					return
				default:
				}
			}
		}()
	}
	for range 4 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			for range 10 {
				tr.ToolResult(ctx, "read_file", "call", strings.Repeat("one complete result\n", 20), false)
			}
		}()
	}
	close(start)
	writers.Wait()
	close(stop)
	readers.Wait()
	if n, err := s.Failures(); n != 0 {
		t.Fatalf("concurrent append failures=%d error=%v", n, err)
	}
	var doc transcriptDoc
	readJSON(t, path, &doc)
	if len(doc.Events) != 41 || doc.Events[40].Seq != 41 {
		t.Fatalf("concurrent writers lost completed events: %d", len(doc.Events))
	}
}

func TestTranscriptRecovery_InitialCreationPreservesLegacyModeSemantics(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(strconv.FormatBool(existing), func(t *testing.T) {
			installStore(t, "session-1", false)
			ctx := WithAgent(context.Background(), "coder", "coder-1")
			tr := TranscriptFrom(ctx)
			control := filepath.Join(filepath.Dir(tr.path), "creation-mode-control")
			if err := os.WriteFile(control, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			mode, err := os.Stat(control)
			if err != nil {
				t.Fatal(err)
			}
			want := mode.Mode().Perm()
			if existing {
				if err := os.WriteFile(tr.path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				mode, err = os.Stat(tr.path)
				if err != nil {
					t.Fatal(err)
				}
				want = mode.Mode().Perm()
			}
			tr.Input(ctx, "created without changing inherited permission settings")
			info, err := os.Stat(tr.path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != want {
				t.Errorf("initial mode = %o, legacy creation mode = %o", info.Mode().Perm(), want)
			}
			var doc transcriptDoc
			readJSON(t, tr.path, &doc)
			if len(doc.Events) != 1 || doc.Events[0].Seq != 1 {
				t.Fatalf("initial snapshot = %+v", doc.Events)
			}
		})
	}
}
