package golp

import (
	"os"
	"log"
	"bufio"
	"path/filepath"
	"github.com/howeyc/fsnotify"
)

type SrcOptions struct{
	Cwd string
}

type StreamFile struct{
	Event *fsnotify.FileEvent
	File *os.File
}

type Stream struct{
	Files []*StreamFile
}


type Step func(ss ...*StreamFile) []*StreamFile

func (s *Stream) Src(patterns []string, o SrcOptions) *Stream{

	cwd, _ := os.Getwd()
	// Change to the requested working directory
	if o.Cwd != ""{
		if err := os.Chdir(o.Cwd); err != nil{
			panic(err)
		}
	}

	// Aggregate all our files
	matches := make(map[string]bool)
	for _, p := range patterns {
		// panic(filepath.Join(o.Cwd, s))
		files, err := filepath.Glob(p)
		log.Println("files captured:", files)
		if err != nil{
			panic(err)
		}
		// Dedupe files by using hash
		for _, f := range files{
			matches[f] = true
		}
	}

	// Open files matched
	files := []*StreamFile{}
	for f, _ := range matches{
		file, err := os.Open(f)
		if err != nil{
			panic(err)
		}
		i, err := file.Stat()
		if i.IsDir() == false {
			files = append(files, &StreamFile{File:file})
		}
	}
	s.Files = files

	// Change back to our initial directory
	if err := os.Chdir(cwd); err != nil{
		panic(err)
	}

	return s
}

// Helper function for simple streams
func Src(patterns ...string) (s *Stream){
	s = &Stream{}
	s.Src(patterns, SrcOptions{})
	return
}

// Create a new stream
func New() (s *Stream){
	return &Stream{}
}

func (s *Stream) Pipe(cb Step) *Stream{
	result := cb(s.Files...)
	return &Stream{result}
}

func Dest(p string) Step{
	
	return func(files ...*StreamFile) []*StreamFile{
		for _, f := range files{

			if f.Event != nil{
				switch {
					case f.Event.IsDelete():
						os.Remove(f.File.Name())
					case f.Event.IsRename():
						os.Rename("a", "b")
					case f.Event.IsCreate():
						os.Create(f.File.Name())
						// pipe
					case f.Event.IsModify():
						//pipe
				}
			}

			buf := make([]byte, 1024)
			// _, err := f.File.Read(buf)
			// Write our file to the parent directory

			path := filepath.Join(p, f.File.Name())

			// Set up needed directories
			log.Println("BASE", p, path, filepath.Dir(path))
			cwd, _ := os.Getwd()
			log.Println("CWD", cwd)
			err := os.MkdirAll("hello", 0666)
			if err != nil{
			    panic(err)
			}
			dest, err := os.Create("hello/test.txt")
			if err != nil{
				panic(err)
			}

			w := bufio.NewWriter(dest)
			w.Write(buf)
		}
		return files
	}
}

type StreamChan struct{
	Watcher *fsnotify.Watcher
	Chan chan []*StreamFile
}

func (s *StreamChan) Pipe(cb Step) *StreamChan{
	events := make(chan []*StreamFile, 5)
	go func(){
		for{
			files := <-s.Chan
			log.Println("pulled from channel", files)
			events <- cb(files...)
		}
	}()
	return &StreamChan{Chan:events}
}

func Watch(patterns ...string) *StreamChan{

	// done := make(chan bool, 1)
	events := make(chan []*StreamFile, 5)

	watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Fatal(err)
    }

	// Aggregate all our files
	watches := make(map[string]bool)
	matches := make(map[string]bool)
	for _, p := range patterns {
		// panic(filepath.Join(o.Cwd, s))
		files, err := filepath.Glob(p)
		log.Println("files captured:", files)
		if err != nil{
			panic(err)
		}
		// Dedupe files by using hash
		// Due to the the way most text editors work we watch the dir
		for _, f := range files{
			watches[filepath.Dir(f)] = true
			matches[f] = true
		}
	}

	files := map[string]*StreamFile{}
	for f, _ := range matches{
		file, err := os.Open(f)
		if err != nil{
			panic(err)
		}
		i, err := file.Stat()
		if i.IsDir() == false {
			files[f] = &StreamFile{File:file}
		}
	}

	go func() {
        for {
            select {
            case ev := <-watcher.Event:
            	_, fok := matches[ev.Name]
            	_, dok := matches[filepath.Dir(ev.Name)]
            	// Is ev.File in our list of matches?
                // or is the directory?
            	if fok || dok{
            		files[ev.Name].Event = ev
            		values := []*StreamFile{}
            		for _, f := range files{
            			values = append(values, f)
            		}
            		events <- values
            		files[ev.Name].Event = nil
	            }
            case err := <-watcher.Error:
                log.Println("error:", err)
		        // done <- true
            }
        }
    }()

	for path, _ := range watches{
		if err := watcher.Watch(path); err != nil{
			panic(err)
		}
	}

	return &StreamChan{Watcher: watcher, Chan: events}
	
}