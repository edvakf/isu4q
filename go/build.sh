#!/bin/bash -x

go get github.com/go-martini/martini
go get github.com/go-sql-driver/mysql
go get github.com/martini-contrib/render
go get github.com/martini-contrib/sessions
go get github.com/walf443/stopwatch
go build -o golang-webapp .

pkill golang-webapp
