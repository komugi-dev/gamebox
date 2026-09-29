module rock-paper-scissors

go 1.25.7

require (
	github.com/komugi-dev/gamebox v0.0.1
	github.com/komugi-dev/gamebox/examples/rock-paper-scissors v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/beevik/guid v1.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/komugi-dev/gamebox => ../../

replace github.com/komugi-dev/gamebox/examples/rock-paper-scissors => ../../examples/rock-paper-scissors
