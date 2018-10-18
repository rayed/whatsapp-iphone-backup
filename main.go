package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

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
	JID       *string
	Name      *string
	Text      *string
	MediaItem *int
	Media     Media
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
	app.ManifestDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", mainfestDstFile))
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
	app.ChatDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", chatDstFile))
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
		media.Hash = hashMap[*path]
		media.Ext = filepath.Ext(*path)
		app.MediaMap[id] = media
	}
}

func (app *App) GetSessions() ([]Session, error) {
	query := "SELECT Z_PK, ZCONTACTJID, ZPARTNERNAME FROM ZWACHATSESSION"
	rows, err := app.ChatDB.Query(query)
	check("GetSessions Query", err)

	css := []Session{}
	for rows.Next() {
		cs := Session{}
		if err := rows.Scan(&cs.ID, &cs.CID, &cs.Name); err != nil {
			log.Println("Error:", err)
			return nil, err
		}
		css = append(css, cs)
	}
	return css, nil
}

// func (app *App) ExtractMedia(cid string) ([]Media, error) {
// 	rows, err := app.ManifestDB.Query("SELECT fileID,relativePath FROM Files WHERE relativePath like 'Message/Media/966504469339@s.whatsapp.net/%' ")
// 	if err != nil {
// 		log.Println("Error:", err)
// 		return nil, err
// 	}
// 	medias := []Media{}
// 	media := Media{}
// 	for rows.Next() {
// 		if err := rows.Scan(&media.ID, &media.Path); err != nil {
// 			log.Println("Error:", err)
// 			return nil, err
// 		}
// 		media.Ext = filepath.Ext(media.Path)
// 		medias = append(medias, media)
// 	}
// 	return medias, nil
// }

// func (app *App) CopyMedia(session Session, mediaList []Media) {
// 	dstDir := fmt.Sprintf("%s/media/%s", app.DstDir, session.ID)
// 	os.MkdirAll(dstDir, 0700)
// 	for _, file := range mediaList {
// 		if file.Ext == ".thumb" {
// 			continue
// 		}
// 		src := fmt.Sprintf("%s/%s/%s", app.SrcDir, file.ID[:2], file.ID)
// 		dst := fmt.Sprintf("%s/%s%s", dstDir, file.ID, file.Ext)
// 		copyFile(src, dst)
// 	}
// }

func (app *App) SessionMessages(session Session) []Message {
	query := `
	SELECT ZFROMJID, ZPUSHNAME, ZTEXT, ZMEDIAITEM
	FROM ZWAMESSAGE
	WHERE ZCHATSESSION = ?
	ORDER BY ZSORT 
	`
	rows, err := app.ChatDB.Query(query, session.ID)
	check("SessionMessages", err)

	messages := []Message{}
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.JID, &msg.Name, &msg.Text, &msg.MediaItem); err != nil {
			log.Println("Error:", err)
		}
		EmptyString := "<nil>"
		if msg.Text == nil {
			msg.Text = &EmptyString
		}
		if msg.MediaItem != nil {
			fmt.Println("Media:", *msg.MediaItem)
			msg.Media = app.MediaMap[*msg.MediaItem]
		}
		messages = append(messages, msg)
	}
	return messages
}

func main() {
	fmt.Println("iPhone Extractor")

	srcPtr := flag.String("src", "src", "iPhone backup source directory")
	dstPtr := flag.String("dst", "dst", "WhatsApp dump directory")
	flag.Parse()

	app := NewApp(*srcPtr, *dstPtr)

	app.LoadMediaMap()
	fmt.Println("Media Map:", len(app.MediaMap))

	sessions, err := app.GetSessions()
	check("GetSession", err)
	app.DumpSessions(sessions)

	for _, session := range sessions {
		if session.ID > 5 {
			continue
		}
		// Build Chat Session
		messages := app.SessionMessages(session)
		app.DumpSession(session, messages)
	}
}
