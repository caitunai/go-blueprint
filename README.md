# Go-Blueprint
The project template of Golang.

## How to use
```shell
gonew github.com/caitunai/go-blueprint@v1.7.9 github.com/yourname/project
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
go install github.com/air-verse/air@latest
```
create the configuration and update the content
```shell
cp .app.toml.example .app.toml
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

If it has some issues, try auto fix

```shell
golangci-lint run --fix
```

If it has issues about struct alignment, try this command to fix

```shell
fieldalignment -fix ./path/to/package
```

If the `fieldalignment` command not found, you can install it with this command:
```shell
go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
```

After code style fixed, you can commit the code
```shell
git add .
git commit -m "commit_message"
```

## Build to release
```shell
GOOS=linux GOARCH=amd64 go build
```

**Thanks**
