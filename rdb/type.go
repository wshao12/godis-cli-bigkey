package rdb

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	RDB_MAGIC = "REDIS"

	RDB_6BITLEN  = 0
	RDB_14BITLEN = 1
	RDB_32BITLEN = 0x80
	RDB_64BITLEN = 0x81
	RDB_ENCVAL   = 3

	RDB_ENC_INT8  = 0
	RDB_ENC_INT16 = 1
	RDB_ENC_INT32 = 2
	RDB_ENC_LZF   = 3

	rdb6bitlen  = 1
	rdb14bitlen = 2
	rdb32bitlen = 5
	rdb64bitlen = 9

	rdbenc8len  = 2
	rdbenc16len = 3
	rdbenc32len = 5

	rdbzsetscorelen = 8
)

const (
	RDB_TYPE_STRING = iota
	RDB_TYPE_LIST
	RDB_TYPE_SET
	RDB_TYPE_ZSET
	RDB_TYPE_HASH
	RDB_TYPE_ZSET_2
	RDB_TYPE_MODULE
	RDB_TYPE_MODULE_2
	_
	RDB_TYPE_HASH_ZIPMAP
	RDB_TYPE_LIST_ZIPLIST
	RDB_TYPE_SET_INTSET
	RDB_TYPE_ZSET_ZIPLIST
	RDB_TYPE_HASH_ZIPLIST
	RDB_TYPE_LIST_QUICKLIST
	RDB_TYPE_STREAM_LISTPACKS
)

type readFunc func(f *os.File, l *uint64)

var m = map[int]readFunc{
	RDB_TYPE_STRING:         readString,
	RDB_TYPE_LIST_QUICKLIST: readList,
	RDB_TYPE_SET_INTSET:     readIntSet,
	RDB_TYPE_SET:            readSet,
	RDB_TYPE_ZSET_ZIPLIST:   readZsetZiplist,
	RDB_TYPE_ZSET_2:         readZset,
	RDB_TYPE_HASH_ZIPLIST:   readHashZiplist,
	RDB_TYPE_HASH:           readHash,
}

func readRdbLength(f *os.File, b byte) (len uint64, isInt bool, intLen uint64) {
	flag := (int(b) & 0xC0) >> 6
	if flag == RDB_6BITLEN {
		len = uint64(int(b) & 0x3F)
	} else if flag == RDB_14BITLEN {
		next, _ := ReadBytes(f, 1)
		len = uint64(((int(b) & 0x3F) << 8) | int(next[0]))
	} else if flag == RDB_ENCVAL {
		len, isInt, intLen = readRdbIntLength(f, b)
	} else if b == RDB_32BITLEN {
		next, _ := ReadBytes(f, rdb32bitlen-1)
		len = uint64(binary.LittleEndian.Uint32(next))
	} else if b == RDB_64BITLEN {
		next, _ := ReadBytes(f, rdb64bitlen-1)
		len = binary.LittleEndian.Uint64(next)
	} else {
		panic("Unknown len")
	}
	return

}

func readRdbIntLength(f *os.File, b byte) (len uint64, isInt bool, intLen uint64) {
	flag := (int(b) & 0x03)
	isInt = true
	if flag == RDB_ENC_INT8 {
		next, _ := ReadBytes(f, 1)
		len = uint64(int(next[0]))
		intLen = rdbenc8len - 1
	} else if flag == RDB_ENC_INT16 {
		next, _ := ReadBytes(f, rdbenc16len-1)
		len = uint64(binary.LittleEndian.Uint16(next))
		intLen = rdbenc16len - 1
	} else if flag == RDB_ENC_INT32 {
		next, _ := ReadBytes(f, rdbenc32len-1)
		len = uint64(binary.LittleEndian.Uint32(next))
		intLen = rdbenc32len - 1
	} else if flag == RDB_ENC_LZF {
		isInt = false
		len = readCompressLen(f)
		readOriginalLen(f)
	}
	return
}

func readCompressLen(f *os.File) uint64 {
	b, _ := ReadBytes(f, 1)
	len, _, _ := readRdbLength(f, b[0])
	return len
}

func readOriginalLen(f *os.File) uint64 {
	b, _ := ReadBytes(f, 1)
	len, _, _ := readRdbLength(f, b[0])
	return len
}

func readKey(f *os.File, l uint64) string {
	k, _ := ReadBytes(f, l)
	return string(k)
}

func readString(f *os.File, l *uint64) {
	b, _ := ReadBytes(f, 1)
	length, isInt, intLen := readRdbLength(f, b[0])
	if isInt {
		*l = intLen
		Printf("value:%d\n", length)
	} else {
		v, _ := ReadBytes(f, length)
		*l = length
		Printf("value:%s\n", v)
	}
}

func readList(f *os.File, l *uint64) {
	ncFlag, _ := ReadBytes(f, 1)
	nodeCount, _, _ := readRdbLength(f, ncFlag[0])
	Printf("listCount:%d\n", nodeCount)
	//zipList之后有一个ziplist总体长度的字段
	lenFlag, _ := ReadBytes(f, 1)
	len, _, _ := readRdbLength(f, lenFlag[0])
	*l = len
	Printf("ziplist length:%d\n", len)
	ReadBytes(f, uint64(len))
	//readZiplist(f, int(nodeCount[0]))
}

func readZiplist(f *os.File, n *int) {
	for i := 0; i < *n; i++ {
		zl, _ := ReadBytes(f, 4)
		Println(zl)
		zlbytes := binary.LittleEndian.Uint32(zl)
		Printf("zlbytes:%d", zlbytes)
	}
}

func readIntSet(f *os.File, n *uint64) {
	lenFlag, _ := ReadBytes(f, 1)
	length, isInt, intlen := readRdbLength(f, lenFlag[0])
	if isInt {
		Printf("intset length:%d\n", length)
		*n = intlen
	} else {
		b, _ := ReadBytes(f, length)
		*n = length
		Printf("intset bytes:%v\n", b)
	}
}

func readSet(f *os.File, n *uint64) {
	hashNode, _ := ReadBytes(f, 1)
	Printf("hashNode %d\n", hashNode[0])
	readHashNode(f, int(hashNode[0]), n)
}

func readHashNode(f *os.File, count int, n *uint64) {
	var length uint64
	for i := 0; i < count; i++ {
		lenFlag, _ := ReadBytes(f, 1)
		len, isInt, intLen := readRdbLength(f, lenFlag[0])
		if isInt {
			Printf("hash value:%d\n", len)
			length += intLen
		} else {
			b, _ := ReadBytes(f, len)
			Printf("hash value:%s\n", b)
			length += len
		}
	}
	*n = length
}

func readZsetZiplist(f *os.File, l *uint64) {
	lenFlag, _ := ReadBytes(f, 1)
	len, _, _ := readRdbLength(f, lenFlag[0])
	*l = len
	Printf("zset ziplist length:%d\n", len)
	ReadBytes(f, uint64(len))
}

func readZset(f *os.File, length *uint64) {
	ncFlag, _ := ReadBytes(f, 1)
	nodeCount, _, _ := readRdbLength(f, ncFlag[0])
	Printf("zset skip list node:%d\n", nodeCount)
	var i uint64
	for i = 0; i < nodeCount; i++ {
		lenFlag, _ := ReadBytes(f, 1)
		len, isInt, intLen := readRdbLength(f, lenFlag[0])
		if isInt {
			*length += intLen
		} else {
			ReadBytes(f, uint64(len))
			*length += len
		}
		ReadBytes(f, rdbzsetscorelen)
		*length += rdbzsetscorelen
	}
}

func readHashZiplist(f *os.File, l *uint64) {
	lenFlag, _ := ReadBytes(f, 1)
	len, _, _ := readRdbLength(f, lenFlag[0])
	*l = len
	Printf("hash ziplist length:%d\n", len)
	ReadBytes(f, uint64(len))
}

func readHash(f *os.File, l *uint64) {
	ncFlag, _ := ReadBytes(f, 1)
	nodeCount, _, _ := readRdbLength(f, ncFlag[0])
	Printf("hash dict node:%d\n", nodeCount)
	var i uint64
	for i = 0; i < 2*nodeCount; i++ {
		lenFlag, _ := ReadBytes(f, 1)
		len, isInt, intLen := readRdbLength(f, lenFlag[0])
		if isInt {
			*l += intLen
		} else {
			ReadBytes(f, uint64(len))
			*l += len
		}
	}
}

//Printf wrap fmt.Printf
func Printf(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format, a...)
	}
}

//Println wrap fmt.Println
func Println(a ...interface{}) {
	if DEBUG {
		fmt.Println(a...)
	}
}
