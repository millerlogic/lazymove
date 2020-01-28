package lazymove

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestMover(t *testing.T) {
	basedir, err := ioutil.TempDir("", "lazymove")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(basedir)

	srcdir := basedir + "/src"
	os.Mkdir(srcdir, 0700)
	destdir := basedir + "/dest"
	os.Mkdir(destdir, 0700)

	os.MkdirAll(srcdir+"/a/b/c", 0700)
	ioutil.WriteFile(srcdir+"/a/afile.txt", []byte(`A!`), 0600)
	ioutil.WriteFile(srcdir+"/a/b/bfile.txt", []byte(`B!`), 0600)
	ioutil.WriteFile(srcdir+"/a/b/c/cfile.txt", []byte(`C!`), 0600)

	const tUnit = time.Second / 5
	m := &Mover{
		SourceDir:  srcdir,
		DestDir:    destdir,
		Timeout:    1 * tUnit,
		MinFileAge: 2 * tUnit,
		MinDirAge:  3 * tUnit,
		ErrorFunc: func(m *Mover, err error) (resume bool) {
			t.Errorf("ErrorFunc: %v", err)
			return false
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := m.Run(ctx)
		if err != nil && err != context.Canceled {
			t.Errorf("Run returned: %v", err)
		}
	}()

	sleepUnits := func(n float64) {
		d := time.Duration(n * float64(tUnit))
		t.Logf("Sleeping %v units (%v)", n, d)
		time.Sleep(d)
	}

	// Initial check.
	sleepUnits(0.25) // very small
	checkFilesTest(t, destdir, map[string]bool{
		"a": false,
	})
	checkFilesTest(t, srcdir, map[string]bool{
		"a/afile.txt":     true,
		"a/b/bfile.txt":   true,
		"a/b/c/cfile.txt": true,
	})

	sleepUnits(1)
	checkFilesTest(t, destdir, map[string]bool{
		"a": false,
	})

	sleepUnits(1)
	checkFilesTest(t, destdir, map[string]bool{
		"a/afile.txt":     true,
		"a/b/bfile.txt":   true,
		"a/b/c/cfile.txt": true,
	})
	checkFilesTest(t, srcdir, map[string]bool{
		"a/afile.txt":     false,
		"a/b/bfile.txt":   false,
		"a/b/c/cfile.txt": false,
		"a":               true,
		"a/b":             true,
		"a/b/c":           true,
	})

	sleepUnits(3.25) // Need to wait the full dur,
	// because the file moves updated the dir mod time.
	checkFilesTest(t, destdir, map[string]bool{
		"a/afile.txt":     true,
		"a/b/bfile.txt":   true,
		"a/b/c/cfile.txt": true,
	})
	checkFilesTest(t, srcdir, map[string]bool{
		"a":     false,
		"a/b":   false,
		"a/b/c": false,
	})

	// Make sure the src dir itself always exists:
	checkFilesTest(t, srcdir, map[string]bool{
		".": true,
	})
}

func checkFilesTest(t *testing.T, dir string, check map[string]bool) error {
	for x, shouldExist := range check {
		p := path.Join(dir, x)
		_, err := os.Stat(p)
		exists := err == nil
		if shouldExist != exists {
			if err == nil {
				t.Errorf("%s exists but shouldn't", p)
			} else {
				t.Errorf("%s: %v", p, err)
			}
		}
	}
	return nil
}
