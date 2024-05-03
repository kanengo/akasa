package call

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type messageType uint8

const (
	versionMessage messageType = iota
	requestMessage
	responseMessage
	responseError
	cancelMessage
)

type version uint32

const (
	initialVersion version = iota
)

const currentVersion = initialVersion

func writeVersion(w io.Writer, wLock *sync.Mutex) error {
	var msg [4]byte
	binary.LittleEndian.PutUint32(msg[:], uint32(currentVersion))
	return writeFlat(w, wLock, versionMessage, 0, nil, msg[:])
}

// Message 格式
// id   	[8]byte    		--  消息id
// type 	[1]byte  		--  messageType
// length 	[7]byte			--  剩余消息长度
// payload  [length]byte 	--  消息

func writeMessage(c net.Conn, wLock *sync.Mutex, mt messageType, id uint64, extraHdr, payload []byte, flattenLimit int) error {
	lh, lp := len(extraHdr), len(payload)
	size := 16 + lh + lp

	if size > flattenLimit {
		return writeChunked(c, wLock, mt, id, extraHdr, payload)
	}

	return writeFlat(c, wLock, mt, id, extraHdr, payload)
}

// writeChunked 某些操作系统对批量写入有优化
func writeChunked(w io.Writer, wLock *sync.Mutex, mt messageType, id uint64, extraHdr []byte, payload []byte) error {
	// We use an iovec with up to three entries.
	var vec [3][]byte

	lh, lp := len(extraHdr), len(payload)
	var hdr [16]byte
	binary.LittleEndian.PutUint64(hdr[0:], id)
	binary.LittleEndian.PutUint64(hdr[8:], uint64(mt)|(uint64(lh+lp)<<8))

	vec[0] = hdr[:]
	vec[1] = extraHdr
	vec[2] = payload

	buf := net.Buffers(vec[:]) //某些系统和某些连接类型对批量写入会有优化

	wLock.Lock()
	defer wLock.Unlock()
	n, err := buf.WriteTo(w)
	if err == nil && n != 16+int64(lh)+int64(lp) {
		err = fmt.Errorf("partial write")
	}

	return err
}

// writeFlat 拼接 header, extra header 和 payload 成一个 单独的 flat byte slice
func writeFlat(w io.Writer, wlLck *sync.Mutex, mt messageType, id uint64, extraHdr []byte, payload []byte) error {
	lh, lp := len(extraHdr), len(payload)
	data := make([]byte, 16+lh+lp)
	binary.LittleEndian.PutUint64(data[0:], id)
	val := uint64(mt) | (uint64(lh+lp) << 8)
	binary.LittleEndian.PutUint64(data[8:], val)
	copy(data[16:], extraHdr)
	copy(data[16+lh:], payload)

	wlLck.Lock()
	defer wlLck.Unlock()
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = fmt.Errorf("partial write")
	}

	return err
}

func readMessage(r io.Reader) (messageType, uint64, []byte, error) {
	// Read the header.
	const headerSize = 16
	var hdr [headerSize]byte

	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, 0, nil, err
	}

	id := binary.LittleEndian.Uint64(hdr[0:])
	h2 := binary.LittleEndian.Uint64(hdr[8:])
	mt := messageType(h2 & 0xff)
	dataLen := h2 >> 8

	const maxSize = 100 << 20 // 100m
	if dataLen > maxSize {
		return 0, 0, nil, fmt.Errorf("overly large message length: %d", dataLen)
	}

	// Read the payload
	msg := make([]byte, int(dataLen))
	if _, err := io.ReadFull(r, msg); err != nil {
		return 0, 0, nil, err
	}

	return mt, id, msg, nil
}

// getVersion extracts the version number sent by the peer
func getVersion(id uint64, msg []byte) (version, error) {
	if id != 0 {
		return 0, fmt.Errorf("invalid id %d in handshake", id)
	}

	if len(msg) < 4 {
		return 0, fmt.Errorf("version message too short (%d), must be >= 4", len(msg))
	}
	v := binary.LittleEndian.Uint32(msg)

	if v < uint32(currentVersion) {
		return version(v), nil
	}

	return currentVersion, nil
}
