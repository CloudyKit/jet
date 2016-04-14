#!/usr/bin/env bash

go test -run="^$" -bench="B." -c -memprofile pprof.out
go test -run="^$" -bench="B." -memprofile pprof.out -memprofilerate=1
go tool pprof --pdf -"$1" jet.test pprof.out >> out.pdf
rm out.pdf
rm pprof.out