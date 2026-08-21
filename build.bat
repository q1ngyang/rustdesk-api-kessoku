@echo off
setlocal
if not exist resources\admin\NUL (
  echo reviewed admin build is required; see ADMIN-WEB-PROVENANCE.md 1>&2
  exit /b 1
)
rmdir /s /q release
if not exist release\NUL mkdir release
set GO111MODULE=on
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/api --instanceName api --exclude http/controller/admin
if errorlevel 1 exit /b 1
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/admin --instanceName admin --exclude http/controller/api
if errorlevel 1 exit /b 1
go build -trimpath -buildvcs=true -o release/kessoku-api.exe ./cmd
if errorlevel 1 exit /b 1
for /f %%i in ('git rev-parse HEAD') do set KESSOKU_SOURCE_COMMIT=%%i
if not defined KESSOKU_SOURCE_COMMIT exit /b 1
go version -m release\kessoku-api.exe > release\GO-BUILD-INFO.txt
if errorlevel 1 exit /b 1
findstr /L /C:"github.com/q1ngyang/rustdesk-api-kessoku/v2/cmd" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
findstr /L /C:"vcs.revision=%KESSOKU_SOURCE_COMMIT%" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
findstr /L /C:"vcs.modified=false" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
xcopy resources\i18n release\resources\i18n /E /I /Y
xcopy resources\public release\resources\public /E /I /Y
xcopy resources\templates release\resources\templates /E /I /Y
copy resources\version release\resources\version
xcopy resources\admin release\resources\admin /E /I /Y
if errorlevel 1 exit /b 1
xcopy docs release\docs /E /I /Y
xcopy conf release\conf /E /I /Y
copy README.md release\README.md
copy README_EN.md release\README_EN.md
copy SECURITY-MODEL.md release\SECURITY-MODEL.md
copy MIGRATION.md release\MIGRATION.md
copy OPERATOR-RUNBOOK.md release\OPERATOR-RUNBOOK.md
copy ROLLBACK-RUNBOOK.md release\ROLLBACK-RUNBOOK.md
copy WEB-CLIENT-PROVIDER.md release\WEB-CLIENT-PROVIDER.md
copy ADMIN-WEB-PROVENANCE.md release\ADMIN-WEB-PROVENANCE.md
copy RELEASE-CHECKLIST.md release\RELEASE-CHECKLIST.md
copy RELEASE-PROCESS.md release\RELEASE-PROCESS.md
copy RELEASE_STATUS release\RELEASE_STATUS
copy LICENSE release\LICENSE
copy admin-web\LICENSE release\ADMIN-WEB-LICENSE
mkdir release\runtime
mkdir release\data
endlocal
