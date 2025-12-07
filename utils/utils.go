package utils

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"unicode/utf8"

	"github.com/zeebo/blake3"
)

var (
	localIP       string
	localIPUInt32 uint32
	cookie        uint64 = 0
)

// ParseAddr 解析地址
func ParseAddr(addr net.Addr) (ip string, port int, err error) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP.String(), a.Port, nil
	case *net.UDPAddr:
		return a.IP.String(), a.Port, nil
	case *net.IPAddr:
		return a.IP.String(), 0, nil // IPAddr 无端口
	default:
		return "", 0, fmt.Errorf("unsupported address type: %T", addr)
	}
}

// GetLocalIP 获取本机ip地址
func GetLocalIP() string {
	if len(localIP) == 0 {
		localIP = "127.0.0.1"
		addrSlice, err := net.InterfaceAddrs()
		if nil == err {
			for _, addr := range addrSlice {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if nil != ipnet.IP.To4() {
						localIP = ipnet.IP.String()
						break
					}
				}
			}
		}
	}
	return localIP
}

// GenerateUniqueID 生成不会重复的id，主要用于traceID
func GenerateUniqueID() string {
	if localIPUInt32 == 0 {
		localIPUInt32 = IPv4ToUInt32(GetLocalIP())
	}
	reqStr := fmt.Sprintf("%x_%x_%x_%x", time.Now().Unix(), localIPUInt32, os.Getpid(), atomic.AddUint64(&cookie, 1))
	return base64.StdEncoding.EncodeToString([]byte(reqStr))
}

// GenerateUniqueIDV2 根据Key生成的hash，长度为16字节，若输入量 < 1000 万，8 字节 BLAKE3 基本安全。
func GenerateUniqueIDV2(key string) string {
	data := []byte(key + fmt.Sprintf("%d", time.Now().Unix()))
	hash := blake3.Sum256(data)
	shortHash := hash[:32] // 取前32字节

	return hex.EncodeToString(shortHash)
}

// GenerateUniqueIDV3 根据Key生成的hash，长度为16字节，若输入量 < 1000 万，8 字节 BLAKE3 基本安全。
func GenerateUniqueIDV3(key string, length int) string {
	data := []byte(key + fmt.Sprintf("%d", time.Now().Unix()))
	hash := blake3.Sum256(data)
	shortHash := hash[:length]

	return hex.EncodeToString(shortHash)
}

// IPv4ToUInt32 ipv4转uint32类型
func IPv4ToUInt32(ip string) uint32 {
	var sum uint32 = 0

	if bits := strings.Split(ip, "."); len(bits) == 4 {
		b0, _ := strconv.Atoi(bits[0])
		b1, _ := strconv.Atoi(bits[1])
		b2, _ := strconv.Atoi(bits[2])
		b3, _ := strconv.Atoi(bits[3])

		sum += uint32(b0)
		sum += uint32(b1) << 8
		sum += uint32(b2) << 16
		sum += uint32(b3) << 24
	}

	return sum
}

// EncodeURIComponent URI编码
func EncodeURIComponent(s string, excluded []byte) string {
	var b bytes.Buffer
	written := 0

	for i, n := 0, len(s); i < n; i++ {
		c := s[i]

		switch c {
		case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
			continue
		default:
			// Unreserved according to RFC 3986 sec 2.3
			if 'a' <= c && c <= 'z' {
				continue

			}
			if 'A' <= c && c <= 'Z' {
				continue

			}
			if '0' <= c && c <= '9' {
				continue
			}
			if slices.Contains(excluded, c) {
				continue
			}
		}

		b.WriteString(s[written:i])
		fmt.Fprintf(&b, "%%%02X", c)
		written = i + 1
	}

	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}

// MD5 生成字符串的MD5值(32位小写)
func MD5(text string) string {
	data := []byte(text)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// SubString 截取含有多字节字符的字符串
func SubString(s string, start, length int) string {
	// 转换为rune切片
	rs := []rune(s)
	l := len(rs)

	// 处理边界条件
	if start < 0 {
		start = 0
	}
	if start >= l {
		return ""
	}
	if length < 0 || start+length > l {
		length = l - start
	}

	return string(rs[start : start+length])
}

// CalcUtf8StringLen 计算utf8字符长度
func CalcUtf8StringLen(str string) uint64 {
	return uint64(utf8.RuneCountInString(str))
}

// SHA256 计算SHA256
func SHA256(data []byte) string {
	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:])
}

// SQLLike 模拟 SQL LIKE 操作，支持 % 和 _ 通配符
// % 匹配任意数量字符（包括零个）
// _ 匹配单个字符
func SQLLike(pattern, s string) bool {
	// 空模式只匹配空字符串
	if pattern == "" {
		return s == ""
	}

	// 如果模式是 "%"，匹配任何字符串
	if pattern == "%" {
		return true
	}

	// 将模式转换为 rune 切片以便处理 Unicode 字符
	patternRunes := []rune(pattern)
	sRunes := []rune(s)

	return like(patternRunes, sRunes, 0, 0)
}

func like(pattern, s []rune, patternPos, strPos int) bool {
	// 如果模式已经用完
	if patternPos >= len(pattern) {
		return strPos >= len(s) // 字符串也必须用完才算匹配
	}

	// 如果字符串已经用完但模式还有
	if strPos >= len(s) {
		// 只有当剩余模式都是 % 时才可能匹配
		for i := patternPos; i < len(pattern); i++ {
			if pattern[i] != '%' {
				return false
			}
		}
		return true
	}

	switch pattern[patternPos] {
	case '%':
		// 处理 % 通配符:
		// 1. 跳过 % 匹配空字符
		// 2. 尝试匹配一个或多个字符
		return like(pattern, s, patternPos+1, strPos) || // 跳过 % 匹配空字符
			like(pattern, s, patternPos, strPos+1) // 匹配一个字符并继续用 % 匹配
	case '_':
		// _ 必须匹配一个字符
		return like(pattern, s, patternPos+1, strPos+1)
	default:
		// 普通字符必须精确匹配
		if pattern[patternPos] == s[strPos] {
			return like(pattern, s, patternPos+1, strPos+1)
		}
		return false
	}
}

// AreSlicesEqual 判断两个切片是否相同
func AreSlicesEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// IsBigEndian 判断本机是否为大端
func IsBigEndian() bool {
	if binary.LittleEndian.Uint16([]byte{1, 0}) == 1 {
		return false
	}
	return true
}

// IsMulticastIP 判断IP是否为组播地址
func IsMulticastIP(ip net.IP) bool {
	if ip.IsMulticast() {
		return true
	}
	return false
}
