package main

import (
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"
)

func (app *App) DumpSessions(css []Session) {
	tpl := `
	<h1>WhatsApp</h1>

	{{range .}}
		<a href="session_{{ .ID}}.html" target="session">{{ .Name}}</a><br>
	{{end}}
	
	`
	t, err := template.New("foo").Parse(tpl)
	check("DumpSessions template parsing", err)

	out, err := os.Create(path.Join(app.DstDir, "sessions.html"))
	check("DumpSessions creating file", err)
	defer out.Close()

	err = t.Execute(out, css)
	check("DumpSessions executing template", err)
}

func (app *App) DumpSession(session Session, messages []Message) {
	tpl := `
<style><!--
body {
	background:rgb(229,221,213);	
}
.chat {
	width:600px;
	margin:auto;
}
.message {
	margin: 5px;
	padding: 8px;
}
.incoming {
	background:white;
}
.outgoing {
	background:rgb(221,247,200);
	text-align:right;
}
--></style>
<h1>WhatsApp</h1>

<div class="chat">
{{range .}}	
	<p class="message {{if .JID}}incoming{{else}}outgoing{{end}}">
		{{ nl2br .Text }}
		{{ if eq .MediaExt ".jpg" }}
			<img src="{{.Media}}">
		{{ else if eq .MediaExt ".png" }}
			<img src="{{.Media}}">
		{{ else if eq .MediaExt ".mp4" }}
			<video controls>
				<source src="{{.Media}}" type="video/mp4">
			</video>
		{{end}}
	</p>
{{end}}
</div><!-- chat -->
	`
	funcs := template.FuncMap{
		"nl2br": func(text string) template.HTML {
			return template.HTML(strings.Replace(template.HTMLEscapeString(text), "\n", "<br>", -1))
		},
	}
	t, err := template.New("foo").Funcs(funcs).Parse(tpl)
	check("DumpSession template parsing", err)

	out, err := os.Create(path.Join(app.DstDir, fmt.Sprintf("session_%d.html", session.ID)))
	check("DumpSession creating file", err)
	defer out.Close()

	if len(messages) > 30 {
		messages = messages[len(messages)-30:]
	}
	err = t.Execute(out, messages)
	check("DumpSession executing template", err)
}
