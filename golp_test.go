package golp_test

import(
	"os"
	// "log"
	"time"
	"sync/atomic"
	"github.com/calendreco/golp"
	. "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "testing"
)

// func rewrite() golp.Step{
// 	return func(files ...golp.Files) golp.Files{
// 		// for file := files{
// 		// 	// regex replace every url
// 		// }
// 	}
// }

func TestGolp(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Golp Suite")
}

// An atomic counter
// Cribbed from fsnotify
type counter struct {
	val int32
}

func (c *counter) increment() {
	atomic.AddInt32(&c.val, 1)
}

func (c *counter) value() int32 {
	return atomic.LoadInt32(&c.val)
}

func (c *counter) reset() {
	atomic.StoreInt32(&c.val, 0)
}

func streamfilenames(s *golp.Stream) (o []string){
	return filenames(s.Files)
}

func filenames(s []*golp.StreamFile) (o []string){
	for _, f := range s{
		o = append(o, f.File.Name())
	}
	return
}

var _ = Describe("Sourcing files", func(){

	It("Should load files pass in", func(){
		stream := golp.Src("fixtures/a.js")
		Ω(streamfilenames(stream)).Should(Equal([]string{"fixtures/a.js"}))
	})

	It("Should support glob syntax", func(){
		stream := golp.Src("fixtures/*.js")
		Ω(streamfilenames(stream)).Should(Equal([]string{"fixtures/a.js", "fixtures/b.js"}))
	})

	It("Should load files pass in", func(){
		stream := golp.Src("fixtures/*/*")
		test := []string{"fixtures/nested/a.js", "fixtures/nested/c.js"}
		Ω(streamfilenames(stream)).Should(Equal(test))
	})

	It("Should take multiple inputs", func(){
		stream := golp.Src("fixtures/*", "fixtures/nested/a.js")
		test := []string{"fixtures/a.js", "fixtures/b.js", "fixtures/nested/a.js"}
		Ω(streamfilenames(stream)).Should(Equal(test))
	})

	It("Should dedupe multiple inputs", func(){
		stream := golp.Src("fixtures/*", "fixtures/a.js")
		test := []string{"fixtures/a.js", "fixtures/b.js"}
		Ω(streamfilenames(stream)).Should(Equal(test))
	})

	It("Should support new factory", func(){
		stream := golp.New().Src([]string{"fixtures/a.js"}, golp.SrcOptions{})
		test := []string{"fixtures/a.js"}
		Ω(streamfilenames(stream)).Should(Equal(test))
	})

	It("Should respect cwd", func(){
		o := golp.SrcOptions{Cwd: "fixtures"}
		stream := golp.New().Src([]string{"a.js"}, o)
		test := []string{"a.js"}
		Ω(streamfilenames(stream)).Should(Equal(test))
	})

})

// A simply passthrough step that checks the files
// it should recieve
func teststep(match ...string) golp.Step{
	return func(files ...*golp.StreamFile) []*golp.StreamFile{
		Ω(filenames(files)).Should(Equal(match))
		return files
	}
}

var _ = Describe("Piping files", func(){

	It("Should pipe files from src to the next step", func(){
		golp.Src("fixtures/a.js").Pipe(teststep("fixtures/a.js"))
	})

})

var _ = Describe("Writing files", func(){

	BeforeEach(func(){
		err := os.Mkdir("fixtures/dist", 0666)
		if err != nil{
			panic(err)
		}
	})

	AfterEach(func(){
		err := os.RemoveAll("fixtures/dist")
		if err != nil{
			panic(err)
		}
	})

	FIt("Should write files to output", func(){
		golp.Src("fixtures/a.js").Pipe(golp.Dest("fixtures/dist"))
		_, err := os.Open("fixtures/dist/fixtures/a.js")
		Ω(err).Should(BeNil())
	})

	It("Should respect cwd", func(){
		o := golp.SrcOptions{Cwd: "fixtures"}
		golp.New().Src([]string{"a.js"}, o).Pipe(golp.Dest("fixtures/dist"))
		_, err := os.Open("fixtures/dist/a.js")
		Ω(err).Should(BeNil())
	})
})

func testasyncstep(c *counter, match ...string) golp.Step{
	return func(files ...*golp.StreamFile) []*golp.StreamFile{
		c.increment()
		Ω(filenames(files)).Should(Equal(match))
		return files
	}
}

func testdiffstep(c *counter, match ...string) golp.Step{
	return func(files ...*golp.StreamFile) []*golp.StreamFile{
		defer GinkgoRecover()
		for _, f := range files {
			if f.Event != nil && f.Event.IsModify(){
				c.increment()
			}
		}
		Ω(len(files)).Should(Equal(len(match)))
		return files
	}
}

// Write current timestamp to an existing file
func modifyfile(file string){
	// Use Nano to ensure the file actually changes
	text := time.Now().Format(time.RFC3339Nano)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND, 0660)
	if _, err = f.WriteString(text); err != nil {
	    panic(err)
	}
	f.Sync()
	f.Close()
}

var _ = Describe("Watching files", func(){

	// var done chan bool
	var modifyReceived counter

	BeforeEach(func(){
		modifyReceived.reset()
	})

	AfterEach(func(){
		// Clear out the file after chaning
		err := os.Truncate("fixtures/a.js", 0)
		if err != nil{
			panic(err)
		}
	})

	It("Should get all the files being watched", func(){
		golp.Watch("fixtures/a.js").Pipe(testasyncstep(&modifyReceived, "fixtures/a.js"))
		Eventually(modifyReceived.value).Should(BeNumerically("==", 1))
	})

	// The step should be called twice, once on the initial setup
	// and then again on the file modification
	It("Should detect changes", func(){
		golp.Watch("fixtures/a.js").Pipe(testasyncstep(&modifyReceived, "fixtures/a.js"))
		modifyfile("fixtures/a.js")
		Eventually(modifyReceived.value).Should(BeNumerically("==", 2))
	})

	It("Should pass the events and original files", func(){
		f := []string{"fixtures/a.js", "fixtures/b.js"}
		golp.Watch("fixtures/*.js").Pipe(testdiffstep(&modifyReceived, f...))
		modifyfile("fixtures/a.js")
		Eventually(modifyReceived.value).Should(BeNumerically("==", 1))
	})

})

// How do we know to get rid of old entries?
// how do we fetch the right file?

// func buster('buster.json')

// func cory() golp.Step{
// 	lookup := map[string]string{"hi.js": "hi-123.js"}
// 	return func(file ...golp.Files) golp.Files{
// 		for file := range files{
// 			switch {
// 				// If deleting this file, delete the versioned one
// 				case f.Event.IsDelete():
// 					f.Name = lookup[f.Name]
// 				case f.Event.IsRename():
// 					f.Name = f.Name+version+ext
// 				case f.Event.IsCreate():
// 				case f.Event.IsModify():
// 					f.Name = f.Name+md5("stuff")+ext
// 			}
// 		}
// 		return files
// 	}
// }

// 
