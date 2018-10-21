package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	ManifestDB *sql.DB
	ChatDB     *sql.DB
	DstDir     string
	SrcDir     string
	MediaMap   map[int]Media
}

type Session struct {
	ID   int
	CID  string
	Name string
}

type Media struct {
	Hash string
	Path string
	Ext  string
}

type Message struct {
	JID      *string
	Name     *string
	Text     string
	Media    string
	MediaExt string
}

func NewApp(src, dst string) *App {
	var err error
	var query string

	app := &App{}
	app.SrcDir = src
	app.DstDir = dst
	app.MediaMap = make(map[int]Media)

	mainfestDstFile := path.Join(dst, "Manifest.db")
	// Copy Manifest DB
	if _, err := os.Stat(mainfestDstFile); os.IsNotExist(err) {
		mainfestSrcFile := path.Join(src, "Manifest.db")
		_, err = copyFile(mainfestSrcFile, mainfestDstFile)
		check("Copy Manifest DB", err)
	}
	// Open Manifest DB
	app.ManifestDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal=off", mainfestDstFile))
	check("Opening Manifest DB", err)
	err = app.ManifestDB.Ping()
	check("Opening Manifest DB (ping)", err)

	// Copy Chat DB
	chatDstFile := path.Join(dst, "ChatStorage.db")
	if _, err := os.Stat(chatDstFile); os.IsNotExist(err) {
		// Query Manifest for ChatStorage.db
		query = `
		SELECT fileID
		FROM Files
		WHERE relativepath='ChatStorage.sqlite'
		AND domain = 'AppDomainGroup-group.net.whatsapp.WhatsApp.shared'
		`
		rows, err := app.ManifestDB.Query(query)
		check("Query manifest", err)
		rows.Next()
		var fileID string
		err = rows.Scan(&fileID)
		check("Scan", err)
		// Copy ChatStorage
		chatSrcFile := path.Join(src, fileID[:2], fileID)
		_, err = copyFile(chatSrcFile, chatDstFile)
		check("Copy Chat DB", err)
	}
	// Open ChatStorage
	app.ChatDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal=off", chatDstFile))
	check("Opening ChatStorage DB", err)
	err = app.ManifestDB.Ping()
	check("Opening ChatStorage DB (ping)", err)

	return app
}

func (app *App) LoadMediaMap() {
	var err error
	var query string
	var id int
	var hash, path *string

	// Build path to hash map
	// e.g. hashMap["somepath"] = "124324345436645645"
	query = "SELECT fileID,relativePath FROM Files"
	rows, err := app.ManifestDB.Query(query)
	check("MediaMap", err)
	hashMap := map[string]string{}
	for rows.Next() {
		err = rows.Scan(&hash, &path)
		check("Scan 1", err)
		hashMap[*path] = *hash
		// fmt.Println(*path)
	}

	// Build Media Hash
	query = "SELECT Z_PK, ZMEDIALOCALPATH FROM ZWAMEDIAITEM"
	rows, err = app.ChatDB.Query(query)
	check("MediaMap ChatDB", err)

	for rows.Next() {
		err = rows.Scan(&id, &path)
		check("scan 2", err)
		if path == nil {
			continue
		}
		media := Media{}
		media.Path = *path
		media.Ext = filepath.Ext(*path)
		if path != nil && strings.HasPrefix(*path, "/") {
			media.Hash = hashMap["Message"+*path]
		} else {
			media.Hash = hashMap["Message/"+*path]
		}
		app.MediaMap[id] = media
	}
}

func (app *App) GetSessions() ([]Session, error) {
	query := "SELECT Z_PK, ZCONTACTJID, ZPARTNERNAME FROM ZWACHATSESSION ORDER BY ZLASTMESSAGEDATE DESC"
	rows, err := app.ChatDB.Query(query)
	check("GetSessions Query", err)

	css := []Session{}
	for rows.Next() {
		cs := Session{}
		if err := rows.Scan(&cs.ID, &cs.CID, &cs.Name); err != nil {
			log.Println("Error:", err)
			return nil, err
		}
		if strings.HasSuffix(cs.CID, "@status") {
			continue
		}
		css = append(css, cs)
	}
	return css, nil
}

func (app *App) SessionMessages(session Session) []Message {
	var err error
	query := `
	SELECT ZFROMJID, ZPUSHNAME, ZTEXT, ZMEDIAITEM
	FROM ZWAMESSAGE
	WHERE ZCHATSESSION = ?
	ORDER BY ZSORT 
	`
	rows, err := app.ChatDB.Query(query, session.ID)
	check("SessionMessages", err)

	mediaBase := path.Join("media", strconv.Itoa(session.ID))
	err = os.MkdirAll(path.Join(app.DstDir, mediaBase), 0700)
	check("Makedir", err)

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var mediaID *int
		var text *string
		if err := rows.Scan(&msg.JID, &msg.Name, &text, &mediaID); err != nil {
			log.Println("Error:", err)
		}
		if text != nil {
			msg.Text = *text
		}
		if mediaID != nil {
			media := app.MediaMap[*mediaID]
			if media.Hash != "" {
				mediaSrc := path.Join(app.SrcDir, media.Hash[:2], media.Hash)
				mediaDst := path.Join(app.DstDir, mediaBase, fmt.Sprintf("%d%s", *mediaID, media.Ext))
				if _, err := os.Stat(mediaDst); os.IsNotExist(err) {
					_, err := copyFile(mediaSrc, mediaDst)
					check("Copy media", err)
				}
				msg.Media = path.Join(mediaBase, fmt.Sprintf("%d%s", *mediaID, media.Ext))
				msg.MediaExt = media.Ext
			} else {
				// VCARD maybe?
				// log.Println(">", *mediaID, media)
			}
		}
		messages = append(messages, msg)
	}
	return messages
}

func main() {
	fmt.Println("iPhone Extractor")

	srcPtr := flag.String("src", "src", "iPhone backup source directory")
	dstPtr := flag.String("dst", "dst", "WhatsApp dump directory")
	limitPtr := flag.Int("limit", 500, "Limit number of chats to export")
	flag.Parse()

	app := NewApp(*srcPtr, *dstPtr)

	app.LoadMediaMap()
	fmt.Println("Media Map:", len(app.MediaMap))

	sessions, err := app.GetSessions()
	check("GetSession", err)
	app.DumpSessions(sessions)

	counter := 0
	for _, session := range sessions {
		// Build Chat Session
		fmt.Println("Building session:", session.ID, " : ", session.Name)
		messages := app.SessionMessages(session)
		app.DumpSession(session, messages)
		counter++
		if counter > *limitPtr {
			break
		}
	}
}
