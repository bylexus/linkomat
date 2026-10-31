# Linkomat

## Abstract

Linkomat is a small tool to keep track of my web link and notes collection. It should be very simple and fast to use as a user, and only serves to purpose to quickly drop / fetch a link / note I put in there. It is a Local-first, offline-first web application mainly to be used as PWA on a mobile phone.

## Feature whislist

* available as Web app (PWA)
* Web App: Local first, Offline first: all data is available locally and is synced automatically from/to the server.
* Server: simple storage: uses an sqlite db per user to store data
* An entry consists of a title, url, note field, timestamp, tags
* central server offers web ui plus api
* authenticated only. Auth using username/pw, which then issues a short-lived token and a longer-lived refresh token

## Architecture

The system consists of the following components:

* Central server that offers the persistence layer, auth and API
	* keeps a separate sqlite db for each user
	* username / password for login (https on prod)
	* api uses short-living auth-token (15min) and refresh token (30 days)
* Web App that allows searching / entering / editing entries
	* PWA, local-first, offline-first
	* syncs with the server in the background
	* sync atomicity per entry (sync is done on entry-level)
	* no merge conflict handling: last save wins. It is mainly used by a single user, working on one entry at a time

### API Server

* Golang based web server
* sqlite as data storage, one db per user
* GORM as ORM (<https://gorm.io/>)
* Auth:
	* access to api only with valid api token (short-lived)
	* using Authentication: Bearer http header
	* login using username/password to issue a valid token and a longer-lived refresh token
	* tokens are kept in a separate table with an expiration date
	* the refresh token can be used to issue a new access token, which makes the refresh token invalid.
	* issueing a token always creates a refresh token, too
* Sync logic handled by clients: server just gets store / load record calls for entries.
* Conflict handling is not needed: last save wins.
* User management is done in a separate, non-public sqlitedb
* Server executable should offer cli for user management

### Web UI

* VueJS SPA
* Just plain HTML (browser components)/JS: I will implement my own component framework for this app later, so it should be easily replaceable
* TypeScript, vitejs
* PWA so that it can be placed on home screen on mobile
* offline-first, local-first:
	* all data is available on client
	* all stores first go to the local storage, then sync to the server if available
	* re-sync in background in a regular interval
* Auth tokens stored locally in LocalStorage, sent via Authentication: Bearer header
* auto-refreshes token before invalidity
* Features:
  * login/logout
  * home: recent notes list, search funktion, add new note btn
  * note editing view, with auto-save, delete


### Project structure

The project should use the following file structure:

```
.
├── api             # the GO server part (package api)
│   └── go.mod
├── cmd             # the GO entry points (binaries)
│   ├── go.mod
│   └── linkomat-server
│       └── main.go # builds to linkomat-server
├── doc             # documentation goes here
├── go.work         # the GO workspace
├── README.md
├── shared          # shared GO code
│   └── go.mod
└── web             # The web client application
```


## Whishlist

- Markdown in Notes field with html / edit view
- manual sync-now button
- tag-list / selection (search) in search view
- app icon on home screen - also for browsers like duckduck go on android
- sortable home list: by last modified or by AZ, asc/desc
- link checker: go through all links and check which ones still return 200

- WebSocket connection to clients of the same user, to push a sync instead of polling
