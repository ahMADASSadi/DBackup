package security

import (
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	archiveMagic = "DBK1"
	archiveNonce = 12
)

// chunkPlain is the gzip plaintext size of one AES-GCM frame.
var chunkPlain = 1 << 20

// ErrArchive means the file is damaged or the password is wrong.
var ErrArchive = errors.New("backup archive could not be opened")

// SealFile gzip-compresses srcPath and writes an encrypted DBK1 archive to dstPath.
func SealFile(dstPath, srcPath, password string) (err error) {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(dstPath)
		}
	}()
	if err = seal(dst, src, password); err != nil {
		return err
	}
	return dst.Sync()
}

// OpenFile decrypts a DBK1 archive and gunzips it into dstPath.
func OpenFile(dstPath, srcPath, password string) (err error) {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(dstPath)
		}
	}()
	if err = openArchive(dst, src, password); err != nil {
		return err
	}
	return dst.Sync()
}

func seal(dst io.Writer, src io.Reader, password string) error {
	salt := make([]byte, argonSaltLen)
	nonce := make([]byte, archiveNonce)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate archive salt: %w", err)
	}
	if _, err := rand.Read(nonce[:8]); err != nil {
		return fmt.Errorf("generate archive nonce: %w", err)
	}
	if err := writeHeader(dst, salt, nonce); err != nil {
		return err
	}
	aead, err := newAEAD(password, salt, argonTime, argonMemory, argonThreads)
	if err != nil {
		return err
	}
	frame := &sealer{w: dst, aead: aead, nonce: nonce}
	gz, err := gzip.NewWriterLevel(frame, gzip.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err := io.Copy(gz, src); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return frame.finish()
}

func openArchive(dst io.Writer, src io.Reader, password string) error {
	salt, nonce, memory, timeCost, threads, err := readHeader(src)
	if err != nil {
		return ErrArchive
	}
	if !kdfOK(memory, timeCost, threads) {
		return ErrArchive
	}
	aead, err := newAEAD(password, salt, timeCost, memory, threads)
	if err != nil {
		return ErrArchive
	}
	gz, err := gzip.NewReader(&opener{r: src, aead: aead, nonce: nonce})
	if err != nil {
		return ErrArchive
	}
	if _, err := io.Copy(dst, gz); err != nil {
		_ = gz.Close()
		return ErrArchive
	}
	if err := gz.Close(); err != nil {
		return ErrArchive
	}
	return nil
}

func newAEAD(password string, salt []byte, timeCost, memory uint32, threads uint8) (cipher.AEAD, error) {
	key := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, argonKeyLen)
	defer wipe(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func writeHeader(w io.Writer, salt, nonce []byte) error {
	var hdr [13]byte
	copy(hdr[:4], archiveMagic)
	binary.BigEndian.PutUint32(hdr[4:8], argonMemory)
	binary.BigEndian.PutUint32(hdr[8:12], argonTime)
	hdr[12] = argonThreads
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(salt); err != nil {
		return err
	}
	_, err := w.Write(nonce)
	return err
}

func readHeader(r io.Reader) (salt, nonce []byte, memory, timeCost uint32, threads uint8, err error) {
	var hdr [13]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return nil, nil, 0, 0, 0, err
	}
	if string(hdr[:4]) != archiveMagic {
		return nil, nil, 0, 0, 0, ErrArchive
	}
	memory = binary.BigEndian.Uint32(hdr[4:8])
	timeCost = binary.BigEndian.Uint32(hdr[8:12])
	threads = hdr[12]
	salt = make([]byte, argonSaltLen)
	nonce = make([]byte, archiveNonce)
	if _, err = io.ReadFull(r, salt); err != nil {
		return nil, nil, 0, 0, 0, err
	}
	if _, err = io.ReadFull(r, nonce); err != nil {
		return nil, nil, 0, 0, 0, err
	}
	return salt, nonce, memory, timeCost, threads, nil
}

func kdfOK(memory, timeCost uint32, threads uint8) bool {
	return memory > 0 && memory <= 1024*1024 && timeCost > 0 && timeCost <= 10 && threads > 0 && threads <= 8
}

type sealer struct {
	w     io.Writer
	aead  cipher.AEAD
	nonce []byte
	n     uint32
	buf   []byte
}

func (s *sealer) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if len(s.buf) == chunkPlain {
			if err := s.flush(); err != nil {
				return written, err
			}
		}
		space := chunkPlain - len(s.buf)
		take := len(p)
		if take > space {
			take = space
		}
		s.buf = append(s.buf, p[:take]...)
		p = p[take:]
		written += take
	}
	return written, nil
}

func (s *sealer) flush() error {
	if len(s.buf) == 0 {
		return nil
	}
	if s.n == ^uint32(0) {
		return fmt.Errorf("archive has too many frames")
	}
	binary.BigEndian.PutUint32(s.nonce[8:], s.n)
	s.n++
	sealed := s.aead.Seal(nil, s.nonce, s.buf, nil)
	s.buf = s.buf[:0]
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(sealed)))
	if _, err := s.w.Write(size[:]); err != nil {
		return err
	}
	_, err := s.w.Write(sealed)
	return err
}

func (s *sealer) finish() error {
	if err := s.flush(); err != nil {
		return err
	}
	var end [4]byte
	_, err := s.w.Write(end[:])
	return err
}

type opener struct {
	r       io.Reader
	aead    cipher.AEAD
	nonce   []byte
	n       uint32
	pending []byte
	done    bool
}

func (o *opener) Read(p []byte) (int, error) {
	if len(o.pending) == 0 {
		if o.done {
			return 0, io.EOF
		}
		if err := o.next(); err != nil {
			return 0, err
		}
		if o.done {
			return 0, io.EOF
		}
	}
	n := copy(p, o.pending)
	o.pending = o.pending[n:]
	return n, nil
}

func (o *opener) next() error {
	var size [4]byte
	if _, err := io.ReadFull(o.r, size[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n == 0 {
		o.done = true
		return nil
	}
	if n > uint32(chunkPlain)+16 {
		return ErrArchive
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(o.r, buf); err != nil {
		return err
	}
	binary.BigEndian.PutUint32(o.nonce[8:], o.n)
	o.n++
	plain, err := o.aead.Open(nil, o.nonce, buf, nil)
	if err != nil {
		return ErrArchive
	}
	o.pending = plain
	return nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
