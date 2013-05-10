package ramfs

import (
	"os"
	"os/exec"
	"testing"
)

func TestRamfs(t *testing.T) {
	d := "/tmp/ramfstest/mp"
	exec.Command("/bin/fusermount", "-zu", d).Run()
	t.Log("Fusermount done")
	os.MkdirAll(d, 0700)
	err := New(-1, -1).Mount(d)
	if err != nil {
		t.Fatal("Mount ", err)
	}
	t.Log("Starting writes")
	for _, f := range []string{"aaa", "foo", "bar", "baz", "foo/sub/deep"} {
		os.MkdirAll(d+"/"+f, 0700)
	}
	t.Log("ls")
	bs, _ := exec.Command("/bin/ls", "-lR", d).CombinedOutput()
	os.Stdout.Write(bs)
}
