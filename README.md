# Linkomat

## Abstract

Linkomat is a small tool to keep track of my web link and notes collection. It should be very simple and fast to use as a user, and only serves to purpose to quickly drop / fetch a link / note I put in there. It is a Local-first, offline-first web application mainly to be used as PWA on a mobile phone.

## Feature whislist

* available as Web app (PWA) and CLI (terminal app)
* Web App: Local first, Offline first: all data is available locally and is synced automatically from/to the server.
* Server: simple storage: uses an sqlite db per user to store data
* An entry consists of a title, url, note field, timestamp, tags
* central server offers web ui plus api
* cli uses api of the web server
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
* CLI that allows searching / entering / editing entries

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


### CLI

* Golang based
* Auth tokens stored locally in user's home config dir
* online-only: does direct api calls, no offline storage (first version)
* auto-refreshes token on usage
* works with sub-commands, e.g. `linkomat add "title"`, `linkomat search ......`
* edit command opens local editor (e.g. `linkomat edit foo` --> opens vim, watches file changes, saves)

### Project structure

The project should use the following file structure:

```
.
├── api             # the GO server part
│   ├── go.mod
│   └── main.go
├── cli             # the GO cli client
│   └── go.mod
├── doc             # documentation goes here
├── go.work         # the GO workspace
├── README.md
├── shared          # shared GO code
│   └── go.mod
└── web             # The web client application
```


## Whishlist

- Markdown in Notes field with html / edit view
- WebSocket connection to clients of the same user, to push a sync instead of polling
- manual sync-now button
- ~~bug: back-arrow: clears search~~ done (session 18)
- tag-list / selection (search) in search view
- ~~focus on search field when home panel~~ done (session 18)