package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// getenv reads a given environmental variable and returns it's value if present or defval if not present or empty
func getenv(key string, defval string) string {
	val := os.Getenv(key)
	if val == "" {
		return defval
	}
	return val
}

// getenv reads a given environmental variable as integer and returns it's value if present or defval if not present or empty
func getenv_i(key string, defval int) int {
	val := os.Getenv(key)
	if val == "" {
		return defval
	}
	if i32, err := strconv.Atoi(val); err != nil {
		return defval
	} else {
		return i32
	}
}

// getenv reads a given environmental variable as integer and returns it's value if present or defval if not present or empty
func getenv_i64(key string, defval int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return defval
	}
	if i64, err := strconv.ParseInt(val, 10, 64); err != nil {
		return defval
	} else {
		return i64
	}
}

func isAlphaNumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func containsOnlyAlphaNumeric(input string) bool {
	for _, c := range input {
		if !isAlphaNumeric(c) {
			return false
		}
	}
	return true
}

func removeNonAlphaNumeric(input string) string {
	ret := ""
	for _, c := range input {
		if isAlphaNumeric(c) {
			ret += string(c)
		}
	}
	return ret
}

func ExtractPastaId(path string) (string, error) {
	var id string
	i := strings.LastIndex(path, "/")
	if i < 0 {
		id = path
	} else {
		id = path[i+1:]
	}
	if !containsOnlyAlphaNumeric(id) {
		return "", fmt.Errorf("invalid id")
	}
	return id, nil
}

/* Load MIME types file. MIME types file is a simple text file that describes mime types based on file extenstions.
 * The format of the file is
 * EXTENSION = MIMETYPE
 */
func loadMimeTypes(filename string) (map[string]string, error) {
	ret := make(map[string]string, 0)

	file, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return ret, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			continue
		}
		name, value := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
		if name != "" && value != "" {
			ret[name] = value
		}
	}

	return ret, scanner.Err()
}

func takeFirst(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	return arr[0]
}

/* try to determine the mime type by file extension. Returns empty string on failure */
func mimeByFilename(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i < 0 {
		return ""
	}
	extension := filename[i+1:]
	if mime, ok := mimeExtensions[extension]; ok {
		return mime
	}
	return ""
}

/* Extract the remote IP address of the given remote
 * The remote is expected to come from http.Request and contain the IP address plus the port */
func extractRemoteIP(remote string) string {
	// Check if IPv6
	i := strings.Index(remote, "[")
	if i >= 0 {
		j := strings.Index(remote, "]")
		if j <= i {
			return remote
		}
		return remote[i+1 : j]
	}
	i = strings.Index(remote, ":")
	if i > 0 {
		return remote[:i]
	}
	return remote
}
func timeHumanReadable(timestamp int64) string {
	if timestamp < 60 {
		return fmt.Sprintf("%d s", timestamp)
	}

	minutes := timestamp / 60
	seconds := timestamp - (minutes * 60)
	if minutes < 60 {
		return fmt.Sprintf("%d:%d min", minutes, seconds)
	}

	hours := minutes / 60
	minutes -= hours * 60
	if hours < 24 {
		return fmt.Sprintf("%d s", hours)
	}

	days := hours / 24
	hours -= days * 24
	if days > 365 {
		years := float32(days) / 365.0
		return fmt.Sprintf("%.2f years", years)
	} else if days > 28 {
		weeks := days / 7
		if weeks > 4 {
			months := days / 30
			return fmt.Sprintf("%d months", months)
		}
		return fmt.Sprintf("%d weeks", weeks)
	} else {
		return fmt.Sprintf("%d days", days)
	}
}
