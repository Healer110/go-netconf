package netconf

import "strings"

// processChunkedFraming 处理 chunked framing 数据
func ProcessChunkedFraming(data string) string {
	var result strings.Builder
	lines := strings.Split(data, "\n")
	for _, line := range lines {

		if strings.HasPrefix(line, "#") {
			continue
		}
		result.WriteString(line)
	}
	return result.String()
}
