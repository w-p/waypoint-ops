package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// File follows one event file, reading only what was appended since the last read.
type File struct {
	Path string

	// Events holds every event read so far, in file order.
	Events []Event

	// Bad counts lines that could not be parsed. They are skipped.
	Bad int

	// Newer is set when a line carries a newer contract major version than this reader knows.
	// Reading that file stops there, as the contract asks.
	Newer int

	offset int64
}

// Read appends any whole lines written since the last call. A partial last line (a write in
// progress) is left for the next call. A file that shrank was replaced, so it is read again from
// the start.
func (f *File) Read() error {
	if f.Newer > 0 {
		return nil
	}
	fh, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer fh.Close()

	info, err := fh.Stat()
	if err != nil {
		return err
	}
	if info.Size() < f.offset {
		f.offset = 0
		f.Events = nil
		f.Bad = 0
	}
	if info.Size() == f.offset {
		return nil
	}
	if _, err := fh.Seek(f.offset, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(fh)
	if err != nil {
		return err
	}

	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil
	}
	whole := data[:end+1]
	f.offset += int64(len(whole))

	for _, line := range bytes.Split(whole, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			f.Bad++
			continue
		}
		if e.V > Version {
			f.Newer = e.V
			return nil
		}
		if e.V < 1 || e.Type == "" || e.TS.IsZero() {
			f.Bad++
			continue
		}
		f.Events = append(f.Events, e)
	}
	return nil
}

// Problem describes what is wrong with the file for a person, or returns "" when nothing is.
func (f *File) Problem() string {
	if f.Newer > 0 {
		return fmt.Sprintf("%s: contract v%d is newer than this app reads (v%d)", f.Path, f.Newer, Version)
	}
	if f.Bad > 0 {
		return fmt.Sprintf("%s: %d unreadable lines skipped", f.Path, f.Bad)
	}
	return ""
}
