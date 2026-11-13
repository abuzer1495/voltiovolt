module github.com/abuzer1495/voltiovolt/cmd/vlt-gui

go 1.27

toolchain go1.27.4

require (
	github.com/abuzer1495/voltiovolt v0.0.0
	github.com/wailsapp/wails/v3 v3.1.0
)

require (
	git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.4 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.46.0 // indirect
)

// The VLT GUI is a separate module, so that Wails and its cgo code are not
// in the root module (CGO_ENABLED=0 for the root module, OD-03).
replace github.com/abuzer1495/voltiovolt => ../..
