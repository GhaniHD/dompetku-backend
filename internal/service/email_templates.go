package service

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

type Rendered struct {
	Subject     string
	HTML        string
	Text        string
	Unsubscribe string
}

type emailRow struct {
	Label string
	Value string
	Tone  string
}

type emailView struct {
	Preheader  string
	Title      string
	Paragraphs []string
	Code       string
	CodeNote   string
	Rows       []emailRow
	ListTitle  string
	List       []string
	ButtonText string
	ButtonURL  string
	Note       string
	AppURL     string
	UnsubURL   string
	SettingURL string
}

var emailLayout = template.Must(template.New("email").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#F2F1F7;font-family:Arial,Helvetica,sans-serif;color:#2A2440">
<span style="display:none;max-height:0;overflow:hidden;opacity:0">{{.Preheader}}</span>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#F2F1F7;padding:24px 12px">
<tr><td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px">
<tr><td style="padding:4px 4px 16px">
<table role="presentation" cellpadding="0" cellspacing="0"><tr>
<td style="background:#6A3DE8;border-radius:10px;width:36px;height:36px;text-align:center;color:#ffffff;font-weight:bold;font-size:18px">M</td>
<td style="padding-left:10px;font-size:18px;font-weight:bold;color:#1B1530">MyFulus</td>
</tr></table>
</td></tr>
<tr><td style="background:#ffffff;border-radius:18px;padding:28px 26px">
<h1 style="margin:0 0 14px;font-size:21px;line-height:1.3;color:#1B1530">{{.Title}}</h1>
{{range .Paragraphs}}<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#4A4560">{{.}}</p>{{end}}
{{if .Code}}
<div style="margin:18px 0 6px;padding:16px;background:#F1EDFB;border-radius:14px;text-align:center;font-size:32px;letter-spacing:10px;font-weight:bold;color:#4A2BB0;font-family:'Courier New',monospace">{{.Code}}</div>
{{if .CodeNote}}<p style="margin:0 0 12px;font-size:13px;color:#8A84A0;text-align:center">{{.CodeNote}}</p>{{end}}
{{end}}
{{if .Rows}}
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:14px 0;border-collapse:collapse">
{{range .Rows}}<tr><td style="padding:9px 0;border-bottom:1px solid #EEEBF6;font-size:14px;color:#6B6585">{{.Label}}</td><td style="padding:9px 0;border-bottom:1px solid #EEEBF6;font-size:14px;font-weight:bold;text-align:right;color:{{if eq .Tone "good"}}#0E9F6E{{else if eq .Tone "bad"}}#D64545{{else}}#1B1530{{end}}">{{.Value}}</td></tr>{{end}}
</table>
{{end}}
{{if .List}}
{{if .ListTitle}}<p style="margin:16px 0 6px;font-size:14px;font-weight:bold;color:#1B1530">{{.ListTitle}}</p>{{end}}
<ul style="margin:0 0 12px;padding-left:20px">{{range .List}}<li style="margin:0 0 6px;font-size:14px;line-height:1.5;color:#4A4560">{{.}}</li>{{end}}</ul>
{{end}}
{{if .ButtonURL}}
<table role="presentation" cellpadding="0" cellspacing="0" style="margin:20px 0 6px"><tr><td style="background:#6A3DE8;border-radius:999px">
<a href="{{.ButtonURL}}" style="display:inline-block;padding:13px 26px;font-size:15px;font-weight:bold;color:#ffffff;text-decoration:none">{{.ButtonText}}</a>
</td></tr></table>
{{end}}
{{if .Note}}<p style="margin:16px 0 0;font-size:13px;line-height:1.5;color:#8A84A0">{{.Note}}</p>{{end}}
</td></tr>
<tr><td style="padding:16px 8px;font-size:12px;line-height:1.6;color:#8A84A0;text-align:center">
Kamu menerima email ini karena terdaftar di <a href="{{.AppURL}}" style="color:#6A3DE8;text-decoration:none">MyFulus</a>.
{{if .UnsubURL}}<br><a href="{{.UnsubURL}}" style="color:#8A84A0">Berhenti menerima email jenis ini</a> · <a href="{{.SettingURL}}" style="color:#8A84A0">Atur notifikasi email</a>{{end}}
</td></tr>
</table>
</td></tr></table>
</body></html>`))

func renderEmail(subject string, v emailView) Rendered {
	var buf bytes.Buffer
	if err := emailLayout.Execute(&buf, v); err != nil {
		buf.Reset()
		buf.WriteString(template.HTMLEscapeString(strings.Join(v.Paragraphs, "\n")))
	}
	var t strings.Builder
	t.WriteString(v.Title + "\n\n")
	for _, p := range v.Paragraphs {
		t.WriteString(p + "\n\n")
	}
	if v.Code != "" {
		t.WriteString("Kode: " + v.Code + "\n")
		if v.CodeNote != "" {
			t.WriteString(v.CodeNote + "\n")
		}
		t.WriteString("\n")
	}
	for _, r := range v.Rows {
		t.WriteString(fmt.Sprintf("%s: %s\n", r.Label, r.Value))
	}
	if len(v.Rows) > 0 {
		t.WriteString("\n")
	}
	if len(v.List) > 0 {
		if v.ListTitle != "" {
			t.WriteString(v.ListTitle + "\n")
		}
		for _, l := range v.List {
			t.WriteString("- " + l + "\n")
		}
		t.WriteString("\n")
	}
	if v.ButtonURL != "" {
		t.WriteString(v.ButtonText + ": " + v.ButtonURL + "\n\n")
	}
	if v.Note != "" {
		t.WriteString(v.Note + "\n\n")
	}
	t.WriteString("— MyFulus\n")
	if v.UnsubURL != "" {
		t.WriteString("Berhenti menerima email jenis ini: " + v.UnsubURL + "\n")
	}
	return Rendered{Subject: subject, HTML: buf.String(), Text: t.String(), Unsubscribe: v.UnsubURL}
}
