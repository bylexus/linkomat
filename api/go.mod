module github.com/bylexus/linkomat/api

go 1.26.1

require (
	github.com/bylexus/linkomat/shared v0.0.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/crypto v0.55.0
	golang.org/x/term v0.45.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/bylexus/linkomat/shared => ../shared
