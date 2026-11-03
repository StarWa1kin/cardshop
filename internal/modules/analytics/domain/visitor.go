package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// visitorHashLength 是指纹保留的十六进制长度（16 字节），足够避免日内碰撞。
const visitorHashLength = 32

// BuildVisitorHash 生成当日访客指纹。
//
// 入参里的访客标识、IP 与 UA 都不落库，只保留不可逆哈希；salt 取应用密钥、
// statDate 参与计算，因此指纹每天轮换，无法跨天追踪同一访客。
func BuildVisitorHash(salt, statDate, visitorKey, clientIP, userAgent string) string {
	identity := strings.TrimSpace(visitorKey)
	if identity == "" {
		// 没有访客标识（禁用 localStorage、脚本上报）时退化为 IP + UA 指纹。
		identity = strings.TrimSpace(clientIP) + "|" + strings.TrimSpace(userAgent)
	}
	sum := sha256.Sum256([]byte(salt + "|" + statDate + "|" + identity))
	return hex.EncodeToString(sum[:])[:visitorHashLength]
}
