package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishAndRejectTraversal(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	temp, err := local.CreateTemp(id, ".db")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, sum, err := local.Publish(temp, id+"/"+id+".db")
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len("snapshot")) || len(sum) != 64 {
		t.Fatalf("size %d sum %s", size, sum)
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatal("temporary file still exists")
	}
	file, info, err := local.Open(id + "/" + id + ".db")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if info.Size() != size {
		t.Fatal(info.Size())
	}
	for _, rel := range []string{"../etc/passwd", "/etc/passwd", ".tmp/secret", id + "/../../outside.db"} {
		if _, _, err := local.Open(rel); err == nil {
			t.Fatalf("opened %s", rel)
		}
	}
	usage, err := local.Usage()
	if err != nil || usage != size {
		t.Fatalf("usage %d %v", usage, err)
	}
	outside := filepath.Join(dir, "..", "nope")
	if err := local.Remove(outside); err == nil {
		t.Fatal("removed a path outside the backup directory")
	}
}
