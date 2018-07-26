package rpc

import (
	"encoding/binary"
	"errors"
)

const MAX_SIZE = 10214 * 1024

/*
协议头
*/
type ImPackHeader struct {
	Len uint32 // 请求长度，包含协议头，以及协议body
	Cmd uint32 //协议号
	Seq uint32 //请求唯一标识
	Scd uint32 //协程号
	Rcd uint32 //协程号
	Sid uint32 //serverId
	Cid uint64 // 发送人cid
	Uid uint64 //发送人uid
}

func (h *ImPackHeader) Change() {
	tmp := h.Scd
	h.Scd = h.Rcd
	h.Rcd = tmp
}

/**
将uint32转换为byte数组
*/

func Uint32ToByte(i uint32, buff []byte) {
	binary.BigEndian.PutUint32(buff, i)
}

/**
将uint64转换为[]byte
*/
func Uint64ToByte(i uint64, buff []byte) {
	binary.BigEndian.PutUint64(buff, i)
}

/**
将[]byte转换为uint32
*/

func ByteToUInt32(buff []byte) uint32 {
	return binary.BigEndian.Uint32(buff)
}

/**
将[]byte转换为uint64
*/
func ByteToUInt64(buff []byte) uint64 {
	return binary.BigEndian.Uint64(buff)
}

/**
将header转换为[]byte
*/
func (header *ImPackHeader) encode() []byte {
	var buffer [40]byte
	Uint32ToByte(header.Len, buffer[:4])
	Uint32ToByte(header.Cmd, buffer[4:8])
	Uint32ToByte(header.Seq, buffer[8:12])
	Uint32ToByte(header.Scd, buffer[12:16])
	Uint32ToByte(header.Rcd, buffer[16:20])
	Uint32ToByte(header.Sid, buffer[20:24])
	Uint64ToByte(header.Cid, buffer[24:32])
	Uint64ToByte(header.Uid, buffer[32:40])
	return buffer[:39]
}

/*
将byte数组解析为header
*/
func decodeHeader(buff []byte) *ImPackHeader {
	var header ImPackHeader
	header.Len = ByteToUInt32(buff[:4])
	header.Cmd = ByteToUInt32(buff[4:8])
	header.Seq = ByteToUInt32(buff[8:12])
	header.Scd = ByteToUInt32(buff[12:16])
	header.Rcd = ByteToUInt32(buff[16:20])
	header.Sid = ByteToUInt32(buff[20:24])
	header.Cid = ByteToUInt64(buff[24:32])
	header.Uid = ByteToUInt64(buff[32:40])
	return &header
}

/*
* 协议
 */
type ImPack struct {
	Header *ImPackHeader //协议头
	Body   []byte        //协议内容
}

func NewImPack() *ImPack {
	return &ImPack{Header: &ImPackHeader{}}
}

func decode(buffer *[]byte, len uint32) (ImPack, uint32, error) {
	var readSize uint32
	var p ImPack
	if len < 40 {
		return p, readSize, nil
	}
	buff := *buffer
	header := decodeHeader(buff)
	size := header.Len
	if size < 40 || size > MAX_SIZE {
		return p, readSize, errors.New("heade size too long or tool short")
	}
	if size <= len {

		p.Header = header
		p.Body = make([]byte, size-40)
		copy(p.Body, buff[40:size])
		readSize = uint32(size)
	}
	return p, readSize, nil
}

/*编码*/
func (p ImPack) Encode() []byte {
	length := 40
	if p.Body != nil {
		length += len(p.Body)
	}
	p.Header.Len = uint32(length)
	data := p.Header.encode()
	data = append(data[0:40], p.Body[0:len(p.Body)]...)
	return data
}

func (p ImPack) Decode(buffer *[]byte, size uint32) ([]Pack, uint32, error) {
	ps := make([]Pack, 1)
	i := 0
	var useSize uint32 = 0
	tmp := *buffer
	for {
		buff := tmp[useSize:]
		p, length, err := decode(&buff, size-useSize)
		if err != nil {
			return nil, 0, err
		}
		if length > 0 {
			useSize += length
			if i < 1 {
				ps[i] = p
				i++

			} else {
				ps = append(ps, p)
			}

		} else {
			return ps, useSize, nil
		}

	}
}
func (c ImPack) GetUniqueId() interface{} {
	if nil != c.Header {
		return c.Header.Seq
	}
	return nil
}
