# WhatsApp iPhone Backup Tool

Export WhatsApp application data from iPhone backup.

The exporter export media files, pictures, movies, audio, etc ... and also export
chat messages as HTML files.

The exporter works on iTunes backup for iPhone. You need to backup your iPhone using
iTunes and make sure you disable encryption. You can enable it again after exporting
WhatApp.

iTunes keep iPhone backup under the folder:

    $HOME/Library/Application Support/MobileSync/Backup/XXXXX-XXXXX/

The ID part changes depend on your setup.

## Running

    go get github.com/mattn/go-sqlite3
    go build -o exporter *.go
    ./exporter -dst "$HOME/whatsapp-backup" -src ""$HOME/Library/Application Support/MobileSync/Backup/XXXXX-XXXXX/"