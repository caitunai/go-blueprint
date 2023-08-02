# Go-Blueprint
The project template of Golang.

## How to use
```shell
gonew github.com/caitunai/go-blueprint@v1.0.0 github.com/yourname/project
```

## Install gonew
```shell
go install golang.org/x/tools/cmd/gonew@latest
```

## Develop and Run
install air
```shell
go install github.com/cosmtrek/air@latest
```
create the configuration and update the content
```shell
mv .app.toml.example .app.toml
vim .app.toml
```
then run project created by this template
```shell
air serve
```

**Thanks**
