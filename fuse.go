// A simple Ramfs on top of Fuse
package ramfs

import (
	. "github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/raw"
	"os"
	"time"
)

// Mount a filesystem. Returns when the filesystem is ready for use.
// All mounted filesystems should be unmounted with Unmount.
func (fs *FS) Mount(mountpoint string) error {
	os.MkdirAll(mountpoint, 0700)
	conn := NewFileSystemConnector(fs, nil)
	fs.ms = NewMountState(conn)
	err := fs.ms.Mount(mountpoint, nil)
	if err != nil {
		return err
	}
	go fs.ms.Loop()
	fs.ms.WaitMount()
	return nil
}

// Unmounts a filesystem.
func (fs *FS) Unmount() error {
	return fs.ms.Unmount()
}

// Ramfs filesystem
type FS struct {
	DefaultNodeFileSystem
	root *fso
	ms   *MountState
}

func (fs *FS) Root() FsNode {
	return fs.root
}

// Initialize an FS. Alternative to New.
func (fs *FS) Init(uid int, gid int) {
	*fs = FS{}
	fs.root = newNode(nil, true)
	if uid == -1 {
		uid = os.Getuid()
	}
	if gid == -1 {
		gid = os.Getgid()
	}
	fs.root.info.Uid = uint32(uid)
	fs.root.info.Gid = uint32(gid)
}

// Create a new FS ready for mounting. Alternative to Init.
func New(uid, gid int) *FS {
	var fs FS
	fs.Init(uid, gid)
	return &fs
}

type fso struct {
	DefaultFsNode
	data []byte
	info Attr
}

func newNode(parent *fso, isdir bool) *fso {
	node := &fso{}
	now := time.Now()
	node.info.SetTimes(&now, &now, &now)
	node.info.Mode = S_IFDIR | 0700
	if parent != nil {
		node.info.Owner = parent.info.Owner
		parent.Inode().New(isdir, node)
	}
	return node
}

func (n *fso) Deletable() bool {
	return false
}

func (n *fso) Readlink(c *Context) ([]byte, Status) {
	return n.data, OK
}

func (n *fso) Mkdir(name string, mode uint32, context *Context) (FsNode, Status) {
	ch := newNode(n, true)
	ch.info.Mode = mode | S_IFDIR
	n.Inode().AddChild(name, ch.Inode())
	return ch, OK
}

func (n *fso) Unlink(name string, context *Context) (code Status) {
	ch := n.Inode().RmChild(name)
	if ch == nil {
		return ENOENT
	}
	return OK
}

func (n *fso) Rmdir(name string, context *Context) (code Status) {
	return n.Unlink(name, context)
}

func (n *fso) Symlink(name string, content string, context *Context) (FsNode, Status) {
	ch := newNode(n, false)
	ch.info.Mode = S_IFLNK | 0777
	ch.data = []byte(content)
	n.Inode().AddChild(name, ch.Inode())

	return ch, OK
}

func (n *fso) Rename(oldName string, newParent FsNode, newName string, context *Context) (code Status) {
	ch := n.Inode().RmChild(oldName)
	newParent.Inode().RmChild(newName)
	newParent.Inode().AddChild(newName, ch)
	return OK
}

func (n *fso) Link(name string, existing FsNode, context *Context) (newNode FsNode, code Status) {
	n.Inode().AddChild(name, existing.Inode())
	return existing, code
}

func (n *fso) Create(name string, flags uint32, mode uint32, context *Context) (File, FsNode, Status) {
	ch := newNode(n, false)
	ch.info.Mode = mode | S_IFREG

	n.Inode().AddChild(name, ch.Inode())
	return ch.newFile(), ch, OK
}

type fsoFile struct {
	*fso
}

func (f fsoFile) SetInode(*Inode) {
}

func (f fsoFile) InnerFile() File {
	return nil
}

func (f fsoFile) String() string {
	return "file"
}

func (f fsoFile) Read(buf []byte, off int64) (ReadResult, Status) {
	soff := int(off)
	if soff > len(f.data) {
		soff = len(f.data)
	}
	return &ReadResultData{f.data[soff:]}, OK
}

func (f fsoFile) Release() {

}

func (f fsoFile) GetAttr(a *Attr) Status {
	return f.fso.GetAttr(a, f, nil)
}

func (f fsoFile) Fsync(flags int) (code Status) {
	return ENOSYS
}

func (f fsoFile) Utimens(atime *time.Time, mtime *time.Time) Status {
	return ENOSYS
}

func (f fsoFile) Truncate(size uint64) Status {
	return ENOSYS
}

func (f fsoFile) Chown(uid uint32, gid uint32) Status {
	return ENOSYS
}

func (f fsoFile) Chmod(perms uint32) Status {
	return ENOSYS
}

func (f fsoFile) Ioctl(input *raw.IoctlIn) (output *raw.IoctlOut, data []byte, code Status) {
	return nil, nil, ENOSYS
}

func (f fsoFile) Allocate(off uint64, size uint64, mode uint32) (code Status) {
	return ENOSYS
}

func (n fsoFile) Flush() Status {
	return OK
}

func (n fsoFile) Write(data []byte, off int64) (uint32, Status) {
	switch {
	case off > 2*1024*1024*1024:
		return 0, EIO
	case int(off)+len(data) > len(n.data):
		old := n.data
		n.data = make([]byte, int(off)+len(data))
		copy(n.data, old)
		fallthrough
	default:
		copy(n.data[int(off):], data)
	}
	n.info.Size = uint64(len(n.data))
	return uint32(len(data)), OK
}

func (n *fso) newFile() File {
	return fsoFile{fso: n}
}

func (n *fso) Open(flags uint32, context *Context) (file File, code Status) {
	return n.newFile(), OK
}

func (n *fso) GetAttr(fi *Attr, file File, context *Context) (code Status) {
	*fi = n.info
	return OK
}

func (n *fso) Truncate(file File, size uint64, context *Context) (code Status) {
	switch {
	case size > 2*1024*1024*1024:
		return EIO
	case size > uint64(len(n.data)):
		old := n.data
		n.data = make([]byte, int(size))
		copy(n.data, old)
	default:
		n.data = n.data[0:int(size)]
	}
	n.info.Size = uint64(len(n.data))
	return OK
}

func (n *fso) Utimens(file File, atime *time.Time, mtime *time.Time, context *Context) (code Status) {
	c := time.Now()
	n.info.SetTimes(atime, mtime, &c)
	return OK
}

func (n *fso) Chmod(file File, perms uint32, context *Context) (code Status) {
	n.info.Mode = (n.info.Mode ^ 07777) | perms
	now := time.Now()
	n.info.SetTimes(nil, nil, &now)
	return OK
}

func (n *fso) Chown(file File, uid uint32, gid uint32, context *Context) (code Status) {
	n.info.Uid = uid
	n.info.Gid = gid
	now := time.Now()
	n.info.SetTimes(nil, nil, &now)
	return OK
}
