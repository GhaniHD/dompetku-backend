package utils

import "strings"

// DescribeDevice meringkas User-Agent menjadi teks seperti "Chrome di Windows".
func DescribeDevice(ua string) string {
	if ua == "" {
		return "Perangkat tidak dikenal"
	}
	browser := "Browser"
	for _, b := range []struct{ token, name string }{
		{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"SamsungBrowser/", "Samsung Internet"},
		{"Firefox/", "Firefox"}, {"CriOS/", "Chrome"}, {"Chrome/", "Chrome"}, {"Safari/", "Safari"},
	} {
		if strings.Contains(ua, b.token) {
			browser = b.name
			break
		}
	}
	os := "perangkat lain"
	for _, o := range []struct{ token, name string }{
		{"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Android", "Android"}, {"Windows", "Windows"},
		{"Mac OS X", "macOS"}, {"CrOS", "ChromeOS"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, o.token) {
			os = o.name
			break
		}
	}
	return browser + " di " + os
}
