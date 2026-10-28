package service

import (
	"context"
	"errors"
	"net"
	"net/mail"
	"strings"
	"time"
)

// AddressError: alamat email ditolak saat daftar atau ganti email.
type AddressError struct{ msg string }

func (e *AddressError) Error() string { return e.msg }

var commonDomains = []string{
	"gmail.com", "yahoo.com", "yahoo.co.id", "outlook.com", "hotmail.com", "icloud.com",
	"live.com", "ymail.com", "proton.me", "protonmail.com", "aol.com",
}

// legitDomains: domain asli yang mirip domain populer, jangan disarankan diganti.
var legitDomains = map[string]bool{
	"mail.com": true, "email.com": true, "gmx.com": true, "live.co.uk": true, "yahoo.co.uk": true,
	"yahoo.co.jp": true, "hotmail.co.uk": true, "me.com": true, "mac.com": true, "aol.co.uk": true,
}

var domainTypos = map[string]string{
	"gmial.com": "gmail.com", "gmai.com": "gmail.com", "gamil.com": "gmail.com", "gmail.co": "gmail.com",
	"gmail.con": "gmail.com", "gmail.cm": "gmail.com", "gmal.com": "gmail.com", "gmaill.com": "gmail.com",
	"gnail.com": "gmail.com", "gmail.co.id": "gmail.com", "gmail.id": "gmail.com", "gmail.om": "gmail.com",
	"yahoo.co": "yahoo.com", "yaho.com": "yahoo.com", "yahooo.com": "yahoo.com", "yahoo.con": "yahoo.com",
	"yahoo.co.di": "yahoo.co.id", "hotmail.co": "hotmail.com", "hotmial.com": "hotmail.com",
	"outlok.com": "outlook.com", "outlook.co": "outlook.com", "icloud.co": "icloud.com", "iclod.com": "icloud.com",
}

var disposableDomains = map[string]bool{
	"mailinator.com": true, "guerrillamail.com": true, "guerrillamail.net": true, "sharklasers.com": true,
	"10minutemail.com": true, "10minutemail.net": true, "temp-mail.org": true, "tempmail.com": true,
	"tempmail.net": true, "tempmailo.com": true, "throwawaymail.com": true, "yopmail.com": true,
	"yopmail.net": true, "getnada.com": true, "trashmail.com": true, "dispostable.com": true,
	"maildrop.cc": true, "mintemail.com": true, "fakeinbox.com": true, "mohmal.com": true,
	"emailondeck.com": true, "spamgourmet.com": true, "mailnesia.com": true, "tempail.com": true,
	"tempr.email": true, "discard.email": true, "mailcatch.com": true, "moakt.com": true,
	"burnermail.io": true, "temp-mail.io": true, "emailfake.com": true, "linshiyouxiang.net": true,
	"1secmail.com": true, "1secmail.net": true, "1secmail.org": true, "mail.tm": true, "inboxkitten.com": true,
}

// levenshtein1 bernilai true bila dua string berbeda tepat satu huruf
// (ganti, sisip, hapus, atau tukar dua huruf bersebelahan).
func levenshtein1(a, b string) bool {
	if a == b {
		return false
	}
	la, lb := len(a), len(b)
	if la == lb {
		diff := []int{}
		for i := 0; i < la; i++ {
			if a[i] != b[i] {
				diff = append(diff, i)
			}
		}
		if len(diff) == 1 {
			return true
		}
		return len(diff) == 2 && diff[1] == diff[0]+1 && a[diff[0]] == b[diff[1]] && a[diff[1]] == b[diff[0]]
	}
	if la-lb == 1 || lb-la == 1 {
		if la < lb {
			a, b = b, a
		}
		for i := 0; i < len(b); i++ {
			if a[i] != b[i] {
				return a[i+1:] == b[i:]
			}
		}
		return true
	}
	return false
}

func suggestDomain(domain string) string {
	if fix, ok := domainTypos[domain]; ok {
		return fix
	}
	if legitDomains[domain] {
		return ""
	}
	for _, d := range commonDomains {
		if d == domain {
			return ""
		}
	}
	for _, d := range commonDomains {
		if levenshtein1(domain, d) {
			return d
		}
	}
	return ""
}

var lookupMX = net.DefaultResolver.LookupMX
var lookupHost = net.DefaultResolver.LookupHost

func domainResolves(ctx context.Context, domain string) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	mx, err := lookupMX(ctx, domain)
	if err == nil && len(mx) > 0 {
		return !(len(mx) == 1 && mx[0].Host == ".")
	}
	var dnsErr *net.DNSError
	if err != nil && !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) {
		return true
	}
	_, err = lookupHost(ctx, domain)
	if err == nil {
		return true
	}
	return !(errors.As(err, &dnsErr) && dnsErr.IsNotFound)
}

// CheckEmailAddress memeriksa format, salah ketik domain populer, email
// sementara, dan (bila checkDNS) apakah domain bisa menerima email. Bila DNS
// tidak bisa dihubungi, alamat dianggap lolos agar pendaftaran tidak macet.
func CheckEmailAddress(ctx context.Context, email string, checkDNS bool) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || strings.Count(email, "@") != 1 {
		return &AddressError{"format email tidak valid"}
	}
	at := strings.LastIndex(email, "@")
	domain := email[at+1:]
	if at == 0 || !strings.Contains(domain, ".") || strings.HasSuffix(domain, ".") {
		return &AddressError{"format email tidak valid"}
	}
	if fix := suggestDomain(domain); fix != "" {
		return &AddressError{"periksa lagi emailmu, mungkin maksudmu " + email[:at+1] + fix + "?"}
	}
	if disposableDomains[domain] {
		return &AddressError{"email sementara tidak bisa dipakai, gunakan email pribadimu"}
	}
	if checkDNS && !domainResolves(ctx, domain) {
		return &AddressError{"domain email " + domain + " tidak ditemukan, periksa lagi emailmu"}
	}
	return nil
}
