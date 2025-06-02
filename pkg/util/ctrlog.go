/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmail.com
 * @Date    : 2025/05/13
 * @Desc    : 文件操作
 */

package util

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

type LogWatcher struct {
	ctx      context.Context
	Cancel   context.CancelFunc
	filePath string
	callback func(line string)
	lastSize int64
	file     *os.File
	scanner  *bufio.Scanner
	watcher  *fsnotify.Watcher
}

func NewLogWatcher(ctx context.Context, filePath string, callback func(line string)) (*LogWatcher, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		klog.Errorf("failed to get file info: %v", err)
		return nil, err
	}

	// open file and keep it open
	var file *os.File
	for range 100 {
		file, err = os.Open(filePath)
		if err != nil {
			klog.Errorf("failed to open file: %v", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}
		break
	}
	if file == nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	// create scanner
	scanner := bufio.NewScanner(file)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %v", err)
	}

	// watch directory
	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		return nil, fmt.Errorf("failed to add directory watcher: %v", err)
	}

	// watch file
	if err := watcher.Add(filePath); err != nil {
		return nil, fmt.Errorf("failed to add file watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	return &LogWatcher{
		ctx:      ctx,
		Cancel:   cancel,
		filePath: filePath,
		callback: callback,
		lastSize: 0,
		file:     file,
		scanner:  scanner,
		watcher:  watcher,
	}, nil
}

func (f *LogWatcher) Watch() (err error) {
	// ensure file will be closed
	defer f.file.Close()
	defer f.watcher.Close()

	klog.Infof("new log watcher of %s started", f.filePath)

	// read last lines of file
	if f.lastSize >= 0 {
		if err := f.readNewLines(); err != nil {
			klog.Errorf("failed to read initial lines: %v", err)
		}
	}

	for {
		select {
		case <-f.ctx.Done():
			klog.Infof("log watcher of %s context done, exit", f.filePath)
			return nil
		case event := <-f.watcher.Events:
			// check if it is the target file event
			if event.Name != f.filePath {
				continue
			}
			switch {
			case event.Op&fsnotify.Write == fsnotify.Write:
				if err := f.readNewLines(); err != nil {
					klog.Errorf("failed to read new lines: %v", err)
				}
			case event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename:
				klog.Infof("file %s was removed or renamed, waiting for recreation", f.filePath)
				// wait for file recreation
				for {
					if _, err := os.Stat(f.filePath); err == nil {
						// file was recreated, reopen and watch
						if err := f.reopenFile(); err != nil {
							klog.Errorf("failed to reopen file: %v", err)
							continue
						}
						break
					}
					select {
					case <-f.ctx.Done():
						return nil
					default:
						time.Sleep(time.Second)
					}
				}
			}
		case err := <-f.watcher.Errors:
			klog.Errorf("watcher error: %v", err)
			return fmt.Errorf("watcher error: %v", err)
		}
	}
}

// readNewLines read new lines of file
func (f *LogWatcher) readNewLines() error {
	// get current file info
	fileInfo, err := os.Stat(f.filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}
	currentSize := fileInfo.Size()
	// if file size not changed, no need to read
	if currentSize <= f.lastSize {
		return nil
	}
	// seek file from last read position
	if _, err := f.file.Seek(f.lastSize, 0); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}

	// reset scanner to reuse
	f.scanner = bufio.NewScanner(f.file)

	// read new lines
	for f.scanner.Scan() {
		line := f.scanner.Text()
		f.callback(line)
	}

	if err := f.scanner.Err(); err != nil {
		return fmt.Errorf("error while scanning file: %v", err)
	}

	// update last read size
	f.lastSize = currentSize
	return nil
}

// reopenFile reopen file and set watcher
func (f *LogWatcher) reopenFile() error {
	// close old file and watcher
	f.file.Close()
	f.watcher.Remove(f.filePath)

	// reopen file
	file, err := os.Open(f.filePath)
	if err != nil {
		return fmt.Errorf("failed to reopen file: %v", err)
	}

	// recreate scanner
	scanner := bufio.NewScanner(file)

	// re-add watcher
	if err := f.watcher.Add(f.filePath); err != nil {
		file.Close()
		return fmt.Errorf("failed to re-add file watcher: %v", err)
	}

	// update file handle and scanner
	f.file = file
	f.scanner = scanner
	f.lastSize = 0

	klog.Infof("file %s reopened and watching resumed", f.filePath)
	return nil
}
