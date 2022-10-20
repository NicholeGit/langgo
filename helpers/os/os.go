package helper_os

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func GetGoroutineId() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func CreateFolder(p string, ignoreExists bool) error {
	if FolderExists(p) == true && ignoreExists == false {
		err := errors.New("folder exists")
		return err
	}
	if FolderExists(p) == false {
		err := os.MkdirAll(p, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func FolderExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if info == nil {
		return false
	}
	return info.IsDir()
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if info == nil {
		return false
	}
	return true
}

// MoveFile move from src to dst until either an error occurs.
// It returns the number of bytes
// move and the first error encountered while moving, if any.
//
// A successful MoveFile returns err == nil, not err == EOF.
// Because MoveFile is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
func MoveFile(source string, dest string) (written int64, err error) {
	written, err = CopyFile(source, dest)
	if err != nil {
		return written, err
	}
	err = os.Remove(source)
	if err != nil {
		return written, fmt.Errorf("failed removing original file: %s", err)
	}
	return written, nil
}

type ProgressEventType int

const (
	// TransferStartedEvent transfer started, set TotalBytes
	TransferStartedEvent ProgressEventType = 1 + iota
	// TransferDataEvent transfer data, set ConsumedBytes anmd TotalBytes
	TransferDataEvent
	// TransferCompletedEvent transfer completed
	TransferCompletedEvent
	// TransferFailedEvent transfer encounters an error
	TransferFailedEvent
)

type ProgressEvent struct {
	ConsumedBytes int64
	TotalBytes    int64
	EventType     ProgressEventType
}

type IOProgressListener interface {
	ProgressChanged(event *ProgressEvent)
}

// CopyFile copies from src to dst until either an error occurs.
// It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful CopyFile returns err == nil, not err == EOF.
// Because CopyFile is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
func CopyFile(source string, dest string) (written int64, err error) {
	sourceFile, err := os.Open(source)
	if err != nil {
		return written, fmt.Errorf("couldn't open source file: %s", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return written, fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer destFile.Close()
	written, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return written, fmt.Errorf("writing to output file failed: %s", err)
	}
	return written, nil
}

var ErrInvalidWrite = errors.New("invalid write result")

// CopyFileWatcher is identical to CopyBuffer except that it provided listener (if one is required).
func CopyFileWatcher(source string, dest string, buf []byte, listener IOProgressListener) (written int64, err error) {
	var sourceSize int64 = 0
	defer func() {
		if listener != nil {
			if err != nil {
				listener.ProgressChanged(&ProgressEvent{
					ConsumedBytes: written,
					TotalBytes:    sourceSize,
					EventType:     TransferFailedEvent,
				})
			} else {
				listener.ProgressChanged(&ProgressEvent{
					ConsumedBytes: written,
					TotalBytes:    sourceSize,
					EventType:     TransferCompletedEvent,
				})
			}
		}
	}()

	if buf != nil && len(buf) == 0 {
		err = errors.New("empty buffer in CopyFileWatcher")
		return written, err
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		return written, fmt.Errorf("couldn't open source file: %s", err)
	}
	defer sourceFile.Close()

	sourceStat, err := sourceFile.Stat()
	sourceSize = sourceStat.Size()
	if err != nil {
		return written, fmt.Errorf("source file stat: %s", err)
	}

	if listener != nil {
		listener.ProgressChanged(&ProgressEvent{
			ConsumedBytes: 0,
			TotalBytes:    sourceSize,
			EventType:     TransferStartedEvent,
		})
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return written, fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer destFile.Close()

	for {
		nr, er := sourceFile.Read(buf)
		if nr > 0 {
			nw, ew := destFile.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = ErrInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		if listener != nil {
			listener.ProgressChanged(&ProgressEvent{
				ConsumedBytes: written,
				TotalBytes:    sourceStat.Size(),
				EventType:     TransferDataEvent,
			})
		}
	}

	return written, err
}

// MoveFileWatcher is identical to CopyFileWatcher except that it remove the source file when completes
func MoveFileWatcher(source string, dest string, buf []byte, listener IOProgressListener) (written int64, err error) {
	written, err = CopyFileWatcher(source, dest, buf, listener)
	if err != nil {
		return written, err
	}
	err = os.Remove(source)
	if err != nil {
		return written, fmt.Errorf("failed removing original file: %s", err)
	}
	return written, nil
}

func ReadDir(name string, ignoreDotFiles bool) (files []os.DirEntry, err error) {
	src, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	for _, f := range src {
		if ignoreDotFiles && strings.HasPrefix(f.Name(), ".") {
			continue
		}
		files = append(files, f)
	}
	return files, nil
}
