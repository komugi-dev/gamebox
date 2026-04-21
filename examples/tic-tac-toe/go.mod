module tic-tac-toe

go 1.25.1

require github.com/sirupsen/logrus v1.9.4

require github.com/komugi-dev/gamebox v0.0.0-20260421090954-5172456726d0

require (
	github.com/gorilla/mux v1.8.1 // indirect
	golang.org/x/sys v0.34.0 // indirect
)

replace github.com/komugi-dev/gamebox => ../../
