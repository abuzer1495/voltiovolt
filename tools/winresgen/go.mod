module github.com/abuzer1495/voltiovolt/tools/winresgen

go 1.27

toolchain go1.27.4

require (
	github.com/abuzer1495/voltiovolt v0.0.0
	github.com/josephspurrier/goversioninfo v1.7.0
)

require github.com/akavel/rsrc v0.10.2 // indirect

replace github.com/abuzer1495/voltiovolt => ../..
