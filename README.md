# Go-Blueprint
The project template of Golang.

## How to use
```shell
gonew github.com/caitunai/go-blueprint@v1.9.10 github.com/yourname/project
```

## Install gonew
```shell
go install golang.org/x/tools/cmd/gonew@latest
```

## ⚠️ Update the hardcode
> ⚠️ You need to edit the `.golangci.yaml` file for `depguard` settings.
>
> ⚠️ Remember to replace the go module name in this `.golangci.yaml`:
>
> Replace [github.com/caitunai/go-blueprint](https://github.com/caitunai/go-blueprint/blob/main/.golangci.yaml#L93) to `github.com/yourname/project`.

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

Absolutely! Here’s the **updated README tutorial** including Atlas installation instructions:

---

# Database Migration with Atlas

This guide explains how to install Atlas, set up a baseline for an existing MySQL database, and manage schema migrations for multiple environments (`prod`, `dev`, `local`).

---

## 1. Install Atlas

Atlas provides prebuilt binaries for different platforms. Follow the steps below to install it:

### **macOS (Homebrew)**

```bash
brew install ariga/tap/atlas
```

### **Linux (using shell script)**

```bash
curl -sSf https://atlasgo.sh | sh
```

* This downloads and installs the latest Atlas binary in your system.

### **Verify Installation**

```bash
atlas version
```

You should see the installed Atlas version printed.

---
## 2. Configure Environment Variables (`.env`)

Atlas uses environment variables to store sensitive database credentials. Create a `.env` file in your project root:

```env
DB_DEV_USERNAME="dev"
DB_DEV_PASSWORD="devpassword"
DB_DEV_HOST="localhost"
DB_DEV_PORT="3306"
DB_DEV_NAME="devdb"

DB_LOCAL_USERNAME="localuser"
DB_LOCAL_PASSWORD="localpassword"
DB_LOCAL_HOST="localhost"
DB_LOCAL_PORT="3306"
DB_LOCAL_NAME="localdb"

DB_PROD_USERNAME="produser"
DB_PROD_PASSWORD="prodpassword"
DB_PROD_HOST="localhost"
DB_PROD_PORT="3306"
DB_PROD_NAME="productiondb"
```

### Load `.env` in your shell

For **bash/zsh**:

```bash
# Automatically export all variables in .env
set -a
source .env
set +a
```

---

## 3. Generate the Baseline

Inspect your production database and create a baseline schema snapshot:

```bash
atlas schema inspect --env prod > atlas/schema/0-baseline.hcl
```

* Generates `0-baseline.hcl`, capturing the current database structure.
* Baseline represents the starting point for Atlas migration management.

---

## 4. Generate the Initial Migration

Generate an initial migration based on the baseline:

```bash
atlas migrate diff initial --env prod
```

* Creates migration SQL in the `atlas/migrations` directory.
* SQL is generated but not yet applied.

---

## 5. Apply the Baseline

Tell Atlas that the current production database corresponds to the baseline version:

```bash
atlas migrate apply --env prod --baseline 20251120060720
```

* Replace `20251120060720` with the version number from your baseline migration file.
* Atlas records the baseline in the `atlas_schema_migrations` table.
* **No SQL will be executed**, as the database already matches the baseline.

---

## 6. Create and Apply New Migrations

After baseline setup, create new schema migrations. For example, to add auth-related tables:

```bash
atlas migrate diff create_auth_tables --env prod
atlas migrate apply --env prod
```

* First command generates migration SQL reflecting changes.
* Second command applies the migration and updates the migration history.

---

## 7. Switch to Other Environments

Once the baseline is established in `prod`, you can repeat migration steps for `dev` or `local`:

1. Configure environment variables in `atlas.hcl` for each environment.
2. Generate migrations:

```bash
atlas migrate diff <migration_name> --env dev
```

3. Apply migrations:

```bash
atlas migrate apply --env local
```

* Keeps all environments synchronized with the same migration history.

---

### Notes

* Always ensure the baseline file accurately reflects the current database schema.
* Baseline versions are **environment-specific**; each environment must initialize its baseline independently.
* Atlas migration only tracks schema changes; it does **not restore deleted data**. Always back up your database before applying destructive migrations.

---


**Thanks**
