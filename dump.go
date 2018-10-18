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
		<a href="session_{{ .ID}}.html">{{ .Name}}</a><br>
	{{end}}
	
	`
	t, err := template.New("foo").Parse(tpl)
	check("DumpSessions template parsing", err)

	out, err := os.Create(path.Join(app.DstDir, "index.html"))
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
	{{ if .JID }}
		<p class="message incoming">{{ nl2br .Text }}</p>
	{{ else }}
		<p class="message outgoing">{{ nl2br .Text }}</a>
	{{ end }}
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
