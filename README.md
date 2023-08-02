# Go-Blueprint
The project template of Golang.

## How to use
```shell
gonew github.com/caitunai/go-blueprint@v1.4.0 github.com/yourname/project
```

## Install gonew
```shell
go install golang.org/x/tools/cmd/gonew@latest
```

## ⚠️ Update the hardcode
> ⚠️ You need edit the `.golangci.yaml` file for `depguard` settings.
> 
> ⚠️ Do not forget to replace the go module name in this `.golangci.yaml`:
> 
> Replace [github.com/caitunai/go-blueprint](https://github.com/caitunai/go-blueprint/blob/main/.golangci.yaml#L79) to `github.com/yourname/project`.

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

## Lint code and commit
lint the code
```shell
golangci-lint run
```

If has some issues, try auto fix

```shell
golangci-lint run --fix
```

After code style fixed, you can commit the code
```shell
git add .
git commit -m "commit_message"
```

**Thanks**
