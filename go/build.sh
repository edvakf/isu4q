#!/bin/bash -x

go get github.com/go-martini/martini
go get github.com/go-sql-driver/mysql
go get github.com/martini-contrib/render
go get github.com/martini-contrib/sessions
go get github.com/walf443/stopwatch
go get github.com/fzzy/radix
go get github.com/pmylund/go-cache
go build -o golang-webapp .
